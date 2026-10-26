#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

# Keep the complete implementation in memory before checkout can replace run.sh.
main() {
  deploy_reviewed=false
  if [[ ${SSH_ORIGINAL_COMMAND+x} ]]; then
    # authorized_keys may still force ./run.sh without forwarding arguments.
    # Treat the original command as data; never evaluate shell input from SSH.
    if [[ $# != 0 || ! $SSH_ORIGINAL_COMMAND =~ ^deploy\ ([0-9a-f]{40})$ ]]; then
      echo 'Expected SSH command: deploy <40-character commit SHA>' >&2
      exit 2
    fi
    deploy_commit=${BASH_REMATCH[1]}
  elif [[ $# == 1 && $1 =~ ^[0-9a-f]{40}$ ]]; then
    deploy_commit=$1
  elif [[ $# == 2 && $1 == --reviewed && $2 =~ ^[0-9a-f]{40}$ ]]; then
    # Explicit operator deployment of a reviewed PR; the restricted SSH key
    # cannot select this mode through SSH_ORIGINAL_COMMAND.
    deploy_reviewed=true
    deploy_commit=$2
  else
    echo 'Usage: ./run.sh [--reviewed] <40-character commit SHA>' >&2
    exit 2
  fi

  cd "$(dirname "${BASH_SOURCE[0]}")"
  deploy_repo=$(pwd -P)
  for deploy_tool in git docker flock sudo; do
    command -v "$deploy_tool" >/dev/null || { echo "Missing tool: $deploy_tool" >&2; exit 1; }
  done
  if command -v docker-compose >/dev/null; then
    deploy_compose=(docker-compose)
  else
    docker compose version >/dev/null
    deploy_compose=(docker compose)
  fi
  deploy_timeout=${BBS_DEPLOY_HEALTH_TIMEOUT:-120}
  [[ $deploy_timeout =~ ^[1-9][0-9]*$ ]] || { echo 'Invalid health timeout' >&2; exit 2; }
  deploy_nginx_target=${BBS_DEPLOY_NGINX_TARGET:-/etc/nginx/sites-available/bbs.jnsec.org}

  exec 9>"$(git rev-parse --git-path bbs-deploy.lock)"
  flock -n 9 || { echo 'Another deployment is running' >&2; exit 1; }
  if ! git diff --quiet || ! git diff --cached --quiet; then
    echo 'Refusing to overwrite tracked local changes' >&2
    exit 1
  fi
  docker info >/dev/null
  sudo -n true

  if [[ $deploy_reviewed == true ]]; then
    git fetch --no-tags origin "$deploy_commit"
  else
    git fetch --no-tags origin +refs/heads/main:refs/remotes/origin/main
    if [[ $(git rev-parse refs/remotes/origin/main) != "$deploy_commit" ]]; then
      echo 'Refusing stale or non-main deployment; request the current CI commit' >&2
      exit 1
    fi
  fi
  git cat-file -e "$deploy_commit^{commit}"
  deploy_previous_commit=$(git rev-parse HEAD)
  deploy_previous_branch=$(git symbolic-ref --quiet --short HEAD || true)
  deploy_previous_image=
  deploy_previous_container=$(docker ps -aq --filter 'name=^/bbs-app$')
  if [[ -n $deploy_previous_container ]]; then
    deploy_previous_image=$(docker inspect --format '{{.Image}}' bbs-app)
  fi
  deploy_state=$(mktemp -d /tmp/jnsec-bbs-deploy.XXXXXX)
  chmod 700 "$deploy_state"
  deploy_checkout_changed=false
  deploy_app_changed=false
  deploy_nginx_changed=false

  wait_for_app() {
    local expected_image=$1 deadline=$((SECONDS + deploy_timeout)) state
    while (( SECONDS < deadline )); do
      state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}} {{.Image}}' bbs-app) || return 1
      case "$state" in
        "running healthy $expected_image") return 0 ;;
        "running starting $expected_image") ;;
        *) echo "Application is not ready: $state" >&2; return 1 ;;
      esac
      sleep 2
    done
    echo 'Application health check timed out' >&2
    return 1
  }

  finish_deploy() {
    local result=$? rollback_failed=0 path
    trap - EXIT HUP INT TERM
    set +e
    if (( result != 0 )); then
      echo 'Deployment failed; restoring the previous release' >&2
      if [[ $deploy_checkout_changed == true ]]; then
        if [[ -n $deploy_previous_branch ]]; then
          git checkout --quiet "$deploy_previous_branch" || rollback_failed=1
        else
          git checkout --quiet --detach "$deploy_previous_commit" || rollback_failed=1
        fi
      fi
      for path in .env config.yaml deploy/nginx.conf; do
        if [[ -f "$deploy_state/$path" ]]; then
          cp -p "$deploy_state/$path" "$deploy_repo/$path" || rollback_failed=1
        elif [[ -f "$deploy_state/backups-complete" ]]; then
          rm -f "$deploy_repo/$path" || rollback_failed=1
        fi
      done
      if [[ $deploy_nginx_changed == true ]]; then
        if [[ -f "$deploy_state/installed-nginx" ]]; then
          sudo -n cp -p "$deploy_state/installed-nginx" "$deploy_nginx_target" || rollback_failed=1
        else
          sudo -n rm -f "$deploy_nginx_target" || rollback_failed=1
        fi
        sudo -n nginx -t && sudo -n nginx -s reload || rollback_failed=1
      fi
      if [[ -n $deploy_previous_image ]]; then
        docker tag "$deploy_previous_image" bbs:latest || rollback_failed=1
        if [[ $deploy_app_changed == true ]]; then
          "${deploy_compose[@]}" up -d --no-deps --no-build --force-recreate app || rollback_failed=1
          wait_for_app "$deploy_previous_image" || rollback_failed=1
        fi
      elif [[ $deploy_app_changed == true ]]; then
        # A first installation has no previous release to restore.
        docker stop bbs-app || rollback_failed=1
      fi
    fi
    if (( rollback_failed )); then
      echo "Rollback needs operator attention; backups retained in $deploy_state" >&2
      exit 1
    fi
    rm -rf "$deploy_state"
    exit "$result"
  }
  trap finish_deploy EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  mkdir -p "$deploy_state/deploy" "$deploy_state/source"
  for deploy_path in .env config.yaml deploy/nginx.conf; do
    if [[ -f $deploy_path ]]; then
      cp -p "$deploy_path" "$deploy_state/$deploy_path"
    fi
  done
  touch "$deploy_state/backups-complete"
  if [[ -n $deploy_previous_image ]]; then
    docker tag "$deploy_previous_image" bbs:rollback
  fi
  deploy_checkout_changed=true
  git checkout --quiet --detach "$deploy_commit"
  # An archive contains exactly this commit, including no untracked local code,
  # secrets, generated assets or host node_modules.
  git archive "$deploy_commit" | tar -x -C "$deploy_state/source"
  docker build --label "org.opencontainers.image.revision=$deploy_commit" --tag bbs:latest "$deploy_state/source"
  deploy_image=$(docker image inspect --format '{{.Id}}' bbs:latest)

  # Preserve the existing owner-approved configuration synchronization policy.
  cp config.example.yaml config.yaml
  cp .env.example .env
  cp deploy/nginx.conf.example deploy/nginx.conf
  "${deploy_compose[@]}" config --quiet
  deploy_app_changed=true
  "${deploy_compose[@]}" up -d --no-deps --no-build --force-recreate app
  wait_for_app "$deploy_image"

  if ! cmp -s deploy/nginx.conf "$deploy_nginx_target"; then
    if [[ -e $deploy_nginx_target ]]; then
      sudo -n cp -p "$deploy_nginx_target" "$deploy_state/installed-nginx"
    fi
    deploy_nginx_changed=true
    sudo -n cp deploy/nginx.conf "$deploy_nginx_target"
    sudo -n nginx -t
    sudo -n nginx -s reload
  fi
  echo "Deployed and healthy: $deploy_commit"
}

main "$@"
