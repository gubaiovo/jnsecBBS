git pull --ff-only origin main


cp config.example.yaml config.yaml
cp .env.example .env
cp ./deploy/nginx.conf.example ./deploy/nginx.conf

docker-compose build app
docker-compose up -d --no-deps app

i=0
while [ "$(docker inspect -f '{{.State.Health.Status}}' bbs-app 2>/dev/null)" != "healthy" ]; do
  i=$((i + 1))
  if [ "$i" -gt 2 ]; then
    echo "bbs-app 未在超时内变为 healthy，最近日志：" >&2
    docker logs --tail 50 bbs-app >&2 || true
    exit 1
  fi
  sleep 5
done
docker ps -a --filter name=bbs-app


cp deploy/nginx.conf.example deploy/nginx.conf
if ! diff -q deploy/nginx.conf /etc/nginx/sites-available/bbs.jnsec.org >/dev/null; then
  sudo cp deploy/nginx.conf /etc/nginx/sites-available/bbs.jnsec.org
  sudo nginx -t && sudo nginx -s reload
fi
