repo = "frebib/cmd-exporter"
archs = ["amd64", "arm64"]
branches = ["master"]
events = ["push"]
docker_matrix = {
  "alpine": {
    "dockerfile": "Dockerfile",
    "tags": [
      "latest",
      "alpine",
      "%label org.label-schema.version",
    ],
  },
  "debian": {
    "dockerfile": "Dockerfile.debian",
    "tags": [
      "debian",
      "%label org.label-schema.version | %prefix debian",
    ],
  },
}


def main(ctx):
  pipelines = [
      test()
  ]

  for key, vals in docker_matrix.items():
    for arch in archs:
      pipelines.append(docker(key, arch, vals["dockerfile"]))

    if ctx.build.branch in branches and ctx.build.event in events:
      deps = ["docker-%s-%s" % (key, a) for a in archs]
      pipelines.extend(publish(key, deps, vals["tags"]))

  if ctx.build.branch in branches and ctx.build.event in events:
    deps = ["publish-%s-dockerhub" % key for key in docker_matrix.keys()]
    pipelines.append(readme(deps))

  return pipelines

def test():
  return {
    "kind": "pipeline",
    "type": "docker",
    "name": "test",
    "platform": {
      "os": "linux",
    },
    "steps": [
      {
        "name": "go tests",
        "image": "golang:alpine",
        "commands": [
          "go build -o cmd-exporter",
          "go install golang.org/x/tools/cmd/goimports@latest",
          "go install golang.org/x/lint/golint@latest",
          "test -z \"$(gofmt -l . | tee /dev/stderr)\"",
          "test -z \"$(goimports -local -e -d . | tee /dev/stderr)\"",
          "golint ./...",
        ]
      }
    ]
  }

def docker(key, arch, dockerfile):
  return {
    "kind": "pipeline",
    "type": "docker",
    "name": "docker-%s-%s" % (key, arch),
    "depends_on": [
      "test",
    ],
    "platform": {
      "os": "linux",
      "arch": arch,
    },
    "environment": {
      "DOCKER_IMAGE_TOKEN": key,
    },
    "steps": [
      {
        "name": "docker build",
        "pull": "always",
        "image": "registry.spritsail.io/spritsail/docker-build",
        "settings": {
          "dockerfile": dockerfile,
        },
      },
      {
        "name": "docker publish",
        "pull": "always",
        "image": "registry.spritsail.io/spritsail/docker-publish",
        "settings": {
          "registry": {"from_secret": "registry_url"},
          "login": {"from_secret": "registry_login"},
        },
        "when": {
          "branch": branches,
          "event": events,
        },
      },
    ],
  }

def publish(key, deps, tags=[]):
  return [
    {
      "kind": "pipeline",
      "name": "publish-%s-%s" % (key, name),
      "depends_on": deps,
      "platform": {
        "os": "linux",
      },
      "environment": {
        "DOCKER_IMAGE_TOKEN": key,
      },
      "steps": [
        {
          "name": "publish",
          "image": "registry.spritsail.io/spritsail/docker-multiarch-publish",
          "pull": "always",
          "settings": {
            "src_registry": {"from_secret": "registry_url"},
            "src_login": {"from_secret": "registry_login"},
            "dest_registry": registry,
            "dest_repo": repo,
            "dest_login": {"from_secret": login_secret},
            "tags": tags,
          },
          "when": {
            "branch": branches,
            "event": events,
          },
        },
      ],
    }
    for name, registry, login_secret in [
      ("dockerhub", "index.docker.io", "docker_login"),
      ("spritsail", "registry.spritsail.io", "spritsail_login"),
      ("ghcr", "ghcr.io", "ghcr_login"),
    ]
  ]

def readme(deps):
  return {
    "kind": "pipeline",
    "name": "update-readme",
    "depends_on": deps,
    "steps": [
      {
        "name": "dockerhub-readme",
        "pull": "always",
        "image": "jlesage/drone-push-readme",
        "settings": {
          "repo": repo,
          "username": {"from_secret": "docker_username"},
          "password": {"from_secret": "docker_password"},
        },
        "when": {
          "branch": branches,
          "event": events
        }
      }
    ]
  }
