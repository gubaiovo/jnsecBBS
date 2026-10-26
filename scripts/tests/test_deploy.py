"""Exercise deployment ordering and rollback with real Git and mocked services."""

import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
MOCK = r'''
import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import time

tool = Path(sys.argv[0]).name
args = sys.argv[1:]
state_path = Path(os.environ["MOCK_DEPLOY_STATE"])
state = json.loads(state_path.read_text())
scenario = os.environ.get("MOCK_DEPLOY_FAILURE", "")

def save():
    state_path.write_text(json.dumps(state))

def stop(code=0):
    save()
    sys.exit(code)

if tool == "flock":
    try:
        fcntl.flock(int(args[-1]), fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        sys.exit(1)
    sys.exit(0)

if tool == "sleep":
    time.sleep(0.02)
    sys.exit(0)

state["calls"].append([tool] + args)
if tool == "docker" and args[0] == "compose":
    tool, args = "docker-compose", args[1:]

if tool == "docker":
    if args[0] == "info":
        stop()
    if args[0] == "ps":
        if state["container_image"] is not None:
            print("old-container")
        stop()
    if args[0] == "build":
        source = Path(args[-1])
        # Neither untracked local code nor runtime secrets may enter the build.
        assert not (source / "untracked.go").exists()
        assert not (source / ".env").exists()
        state["built_marker"] = (source / "marker").read_text()
        state["built_revision"] = args[args.index("--label") + 1]
        if scenario == "build":
            stop(12)
        state["tags"]["bbs:latest"] = "sha256:new"
        stop()
    if args[0] == "tag":
        image = state["tags"].get(args[1], args[1])
        state["tags"][args[2]] = image
        stop()
    if args[:2] == ["image", "inspect"]:
        print(state["tags"][args[-1]])
        stop()
    if args[0] == "inspect":
        image = state["container_image"]
        if image is None:
            stop(1)
        if args[2] == "{{.Image}}":
            if scenario == "inspect":
                stop(17)
            print(image)
        else:
            state["health_reads"] += 1
            health = "healthy"
            if image == "sha256:new":
                if scenario == "health":
                    health = "unhealthy"
                elif scenario == "timeout":
                    health = "starting"
                elif state["health_reads"] == 1:
                    health = "starting"
            print("running " + health + " " + image)
        stop()
    if args[0] == "stop":
        state["stopped"] = True
        stop()

if tool == "docker-compose":
    if args[0] == "config":
        stop(13 if scenario == "compose-config" else 0)
    if args[0] == "up":
        state["up_count"] += 1
        state["container_image"] = state["tags"]["bbs:latest"]
        if scenario == "up" and state["up_count"] == 1:
            stop(14)
        stop()

if tool == "sudo":
    assert args.pop(0) == "-n"
    if args == ["true"]:
        stop(15 if scenario == "sudo" else 0)
    if args[0] in ("cp", "rm"):
        stop(subprocess.run(["/bin/" + args[0]] + args[1:]).returncode)
    if args[0] == "nginx":
        contents = Path(os.environ["BBS_DEPLOY_NGINX_TARGET"]).read_text()
        if contents == "new nginx" and (
            (args[1:] == ["-t"] and scenario == "nginx-test")
            or (args[1:] == ["-s", "reload"] and scenario == "nginx-reload")
        ):
            stop(16)
        stop()

raise AssertionError("Unexpected command: " + repr([tool] + args))
'''


class DeployTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="bbs-deploy-test-")
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.repo = self.base / "repo"
        self.repo.mkdir()
        self.remote = self.base / "origin.git"
        self.git("init", "--bare", str(self.remote))
        self.git("init", "-b", "main")
        self.git("config", "user.email", "deploy-test@example.invalid")
        self.git("config", "user.name", "Deployment test")
        self.git("remote", "add", "origin", str(self.remote))
        (self.repo / "deploy").mkdir()
        (self.repo / "run.sh").write_bytes((ROOT / "run.sh").read_bytes())
        (self.repo / ".gitignore").write_text(".env\nconfig.yaml\ndeploy/nginx.conf\n")
        (self.repo / ".env.example").write_text("EXAMPLE=new\n")
        (self.repo / "config.example.yaml").write_text("new config")
        (self.repo / "deploy/nginx.conf.example").write_text("new nginx")
        (self.repo / "docker-compose.yaml").write_text("services: {}\n")
        (self.repo / "Dockerfile").write_text("FROM scratch\n")
        (self.repo / "marker").write_text("old")
        self.git("add", ".")
        self.git("commit", "-m", "previous release")
        self.old = self.git("rev-parse", "HEAD")
        (self.repo / "marker").write_text("new")
        self.git("commit", "-am", "next release")
        self.new = self.git("rev-parse", "HEAD")
        self.git("push", "origin", "main")
        self.git("checkout", "--detach", self.old)
        (self.repo / ".env").write_text("EXISTING=local\n")
        (self.repo / "config.yaml").write_text("existing config")
        (self.repo / "deploy/nginx.conf").write_text("existing generated nginx")
        (self.repo / "untracked.go").write_text("package must_not_be_built\n")
        self.nginx = self.base / "nginx.conf"
        self.nginx.write_text("old nginx")
        self.state_path = self.base / "state.json"
        self.state_path.write_text(json.dumps({
            "calls": [], "container_image": "sha256:old", "health_reads": 0,
            "tags": {"bbs:latest": "sha256:old"}, "up_count": 0,
        }))
        self.bin = self.base / "bin"
        self.bin.mkdir()
        mock = self.bin / "mock"
        mock.write_text("#!" + sys.executable + "\n" + MOCK)
        mock.chmod(0o755)
        for name in ("docker", "docker-compose", "sudo", "flock", "sleep"):
            (self.bin / name).symlink_to(mock)
        self.env = dict(os.environ)
        self.env.pop("SSH_ORIGINAL_COMMAND", None)
        self.env.update({
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "MOCK_DEPLOY_STATE": str(self.state_path),
            "BBS_DEPLOY_NGINX_TARGET": str(self.nginx),
            "BBS_DEPLOY_HEALTH_TIMEOUT": "2",
        })

    def git(self, *args):
        result = subprocess.run(["git", *args], cwd=self.repo, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def run_deploy(self, *args, failure="", original_command=None):
        env = dict(self.env, MOCK_DEPLOY_FAILURE=failure)
        if original_command is not None:
            env["SSH_ORIGINAL_COMMAND"] = original_command
        return subprocess.run(["bash", "run.sh", *args], cwd=self.repo, env=env,
                              text=True, stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, timeout=15)

    def state(self):
        return json.loads(self.state_path.read_text())

    def assert_previous_release(self):
        self.assertEqual(self.git("rev-parse", "HEAD"), self.old)
        self.assertEqual((self.repo / ".env").read_text(), "EXISTING=local\n")
        self.assertEqual((self.repo / "config.yaml").read_text(), "existing config")
        self.assertEqual((self.repo / "deploy/nginx.conf").read_text(), "existing generated nginx")
        self.assertEqual(self.nginx.read_text(), "old nginx")
        self.assertEqual(self.state()["container_image"], "sha256:old")
        self.assertEqual(self.state()["tags"]["bbs:latest"], "sha256:old")

    def test_forced_command_deploys_exact_commit_and_waits_for_health(self):
        result = self.run_deploy(original_command="deploy " + self.new)
        self.assertEqual(result.returncode, 0, result.stderr)
        state = self.state()
        self.assertEqual(self.git("rev-parse", "HEAD"), self.new)
        self.assertEqual(state["built_marker"], "new")
        self.assertEqual(state["built_revision"], "org.opencontainers.image.revision=" + self.new)
        self.assertEqual(state["container_image"], "sha256:new")
        self.assertEqual(state["tags"]["bbs:rollback"], "sha256:old")
        self.assertGreaterEqual(state["health_reads"], 2)
        self.assertEqual(self.nginx.read_text(), "new nginx")
        up = next(call for call in state["calls"] if call[:2] == ["docker-compose", "up"])
        self.assertEqual(up[2:], ["-d", "--no-deps", "--no-build", "--force-recreate", "app"])

    def test_stale_ci_commit_does_not_mutate_service(self):
        result = self.run_deploy(self.old)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("stale or non-main", result.stderr)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 0)

    def test_missing_or_malformed_ssh_command_is_rejected(self):
        commands = ("", "deploy main", "deploy " + self.new + "; touch pwned",
                    "deploy --reviewed " + self.new, "deploy " + self.new + "\n")
        for command in commands:
            with self.subTest(command=command):
                self.assertEqual(self.run_deploy(original_command=command).returncode, 2)
        self.assertEqual(self.run_deploy().returncode, 2)
        self.assertEqual(self.state()["calls"], [])
        self.assertFalse((self.repo / "pwned").exists())

    def test_reviewed_commit_is_an_explicit_operator_only_mode(self):
        result = self.run_deploy("--reviewed", self.old)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.state()["built_marker"], "old")
        result = self.run_deploy("--reviewed", self.new, original_command="deploy " + self.new)
        self.assertEqual(result.returncode, 2)

    def test_build_failure_leaves_running_app_and_configuration_intact(self):
        result = self.run_deploy(self.new, failure="build")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 0)

    def test_compose_validation_failure_does_not_replace_running_app(self):
        result = self.run_deploy(self.new, failure="compose-config")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 0)

    def test_partial_up_failure_restores_previous_image_and_environment(self):
        result = self.run_deploy(self.new, failure="up")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 2)

    def test_unhealthy_app_rolls_back(self):
        result = self.run_deploy(self.new, failure="health")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 2)

    def test_health_timeout_rolls_back(self):
        result = self.run_deploy(self.new, failure="timeout")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("timed out", result.stderr)
        self.assert_previous_release()

    def test_invalid_nginx_restores_disk_config_and_app(self):
        result = self.run_deploy(self.new, failure="nginx-test")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertEqual(self.state()["up_count"], 2)

    def test_failed_nginx_reload_restores_disk_config_and_app(self):
        result = self.run_deploy(self.new, failure="nginx-reload")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()

    def test_missing_sudo_permission_fails_before_checkout_or_build(self):
        result = self.run_deploy(self.new, failure="sudo")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertFalse(any(call[:2] == ["docker", "build"] for call in self.state()["calls"]))

    def test_container_inspection_error_is_not_treated_as_first_installation(self):
        result = self.run_deploy(self.new, failure="inspect")
        self.assertNotEqual(result.returncode, 0)
        self.assert_previous_release()
        self.assertFalse(any(call[:2] == ["docker", "build"] for call in self.state()["calls"]))

    def test_first_installation_failure_stops_the_unhealthy_container(self):
        state = self.state()
        state["container_image"] = None
        self.state_path.write_text(json.dumps(state))
        result = self.run_deploy(self.new, failure="health")
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.state()["stopped"])
        self.assertEqual(self.git("rev-parse", "HEAD"), self.old)
        self.assertEqual(self.nginx.read_text(), "old nginx")

    def test_tracked_local_changes_are_preserved(self):
        (self.repo / "marker").write_text("local changes")
        result = self.run_deploy(self.new)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.repo / "marker").read_text(), "local changes")
        self.assertEqual(self.state()["calls"], [])

    def test_repository_lock_prevents_a_second_deployment(self):
        with (self.repo / ".git/bbs-deploy.lock").open("w") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            result = self.run_deploy(self.new)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Another deployment", result.stderr)
        self.assertEqual(self.state()["calls"], [])

    def test_checkout_cannot_replace_the_executing_script(self):
        self.git("checkout", "main")
        (self.repo / "run.sh").write_text("#!/usr/bin/env bash\nexit 99\n")
        self.git("commit", "-am", "replace deployment script")
        self.new = self.git("rev-parse", "HEAD")
        self.git("push", "origin", "main")
        self.git("checkout", "--detach", self.old)
        result = self.run_deploy(self.new)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Deployed and healthy: " + self.new, result.stdout)


if __name__ == "__main__":
    unittest.main()
