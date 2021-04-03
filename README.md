[hub]: https://hub.docker.com/r/frebib/cmd-exporter
[git]: https://github.com/frebib/cmd-exporter
[drone]: https://drone.spritsail.io/frebib/cmd-exporter

# [frebib/cmd-exporter][hub]

[![](https://img.shields.io/docker/image-size/frebib/cmd-exporter/latest)][hub]
[![Latest Version](https://img.shields.io/docker/v/frebib/cmd-exporter?sort=semver)][hub]
[![Git Commit](https://images.microbadger.com/badges/commit/frebib/cmd-exporter.svg)][git]
[![Docker Pulls](https://img.shields.io/docker/pulls/frebib/cmd-exporter.svg)][hub]
[![Docker Stars](https://img.shields.io/docker/stars/frebib/cmd-exporter.svg)][hub]
[![Build Status](https://drone.spritsail.io/api/badges/frebib/cmd-exporter/status.svg)][drone]

A Prometheus exporter for generating metrics from arbitrary commands. This exporter bears a lot of similarities to the textfile_exporter that's part of prometheus/node_exporter, but this differs as it runs the commands inline with the scrape request instead of reading pre-rendered text files.

### Usage
In the simplest form, running the exporter with no flags and only the configuration file is sufficient for most uses.

Grab the binary, and the example config file from github, and start running:
```shell
go get github.com/frebib/cmd-exporter
curl -O https://github.com/frebib/cmd-exporter/blob/master/config.yml
vim config.yml # edit to your liking
cmd-exporter --help
```

Or with Docker, just mount the config file to the working directory (`/` by default). The container will start the exporter with no flags, just the defaults in the binary:
```shell
docker run --rm \
    -v $PWD/config.yml:/config.yml:ro \
    frebib/cmd-exporter
```

### Configuration

The exporter has a few command-line flags that adjust how it runs
```
Usage:
  cmd-exporter

Application Options:
  -l, --listen=       http listen address to serve metrics (default: :9654)
  -p, --metrics-path= http path at which to serve metrics (default: /command)
  -c, --config-file=  path to config yaml file (default: ./config.yml)
```

The config file is YAML and has a few options, but most are pretty simple to understand.
Currently there are only two types of option:
- `startup`: a one-time command to run at exporter startup
- `scripts`: one or more commands/scripts to execute when metrics are scraped

A config file might look something like this
```yaml
startup:
  script: apt -y install python3 lsb-release

scripts:
- name: date
  command: [/usr/bin/env, python3]
  script: |
    import time
    print('# HELP date gets now()')
    print('# TYPE date gauge')
    print('date {}'.format(time.time()))

- name: os-release
  script: |
    . /etc/os-release
    echo "# HELP os_release os release information"
    echo "# TYPE os_release gauge"
    echo "os_release{id=\"$(lsb_release -is)\",description=\"$(lsb_release -ds | tr -d \")\",codename=\"$(lsb_release -cs)\",release=\"$(lsb_release -rs)\"} 1"
```

`startup` is just a normal script. It can have a `command` as well as a `script` block. You could even give it a name if you so wished.

Each script must have at least a `name` and a `command` _or_ a `script`. If the `script` is provided but `command` is omitted then it is set to the default of `[/bin/sh, -e]`. It can be any interpreter you choose, though.
Scripts are piped via stdin to the `command` process, and `stdout` is captured and parsed as metrics to be returned to the caller.

The example above when run in a Debian Bullseye container produces (roughly, trimmed for brevity) the following output:
```
$ docker run --rm -v $PWD/config.yml:/config.yml:ro frebib/cmd-exporter
2021-04-03T00:35:40.284 Starting cmd-exporter, version 0.0.1
2021-04-03T00:35:40.284 running startup command
2021-04-03T00:35:40.611 startup: Reading package lists...
2021-04-03T00:35:40.697 startup: Building dependency tree...
2021-04-03T00:35:40.697 startup: Reading state information...
2021-04-03T00:35:40.771 startup: The following additional packages will be installed:
2021-04-03T00:35:40.771 startup:   ca-certificates distro-info-data libexpat1 libgpm2 libmpdec3 libncursesw6
2021-04-03T00:35:40.771 startup:   libpython3-stdlib libpython3.9-minimal libpython3.9-stdlib libreadline8
2021-04-03T00:35:40.771 startup:   libsqlite3-0 media-types openssl python3-minimal python3.9 python3.9-minimal
2021-04-03T00:35:40.771 startup:   readline-common
2021-04-03T00:35:40.772 startup: Suggested packages:
2021-04-03T00:35:40.772 startup:   gpm python3-doc python3-tk python3-venv python3.9-venv python3.9-doc
2021-04-03T00:35:40.772 startup:   binutils binfmt-support readline-doc
2021-04-03T00:35:40.874 startup: The following NEW packages will be installed:
2021-04-03T00:35:40.875 startup:   ca-certificates distro-info-data libexpat1 libgpm2 libmpdec3 libncursesw6
2021-04-03T00:35:40.875 startup:   libpython3-stdlib libpython3.9-minimal libpython3.9-stdlib libreadline8
2021-04-03T00:35:40.875 startup:   libsqlite3-0 lsb-release media-types openssl python3 python3-minimal
2021-04-03T00:35:40.875 startup:   python3.9 python3.9-minimal readline-common
2021-04-03T00:35:40.897 startup: 0 upgraded, 19 newly installed, 0 to remove and 0 not upgraded.
2021-04-03T00:35:40.897 startup: Need to get 7469 kB of archives.
2021-04-03T00:35:40.897 startup: After this operation, 25.0 MB of additional disk space will be used.
2021-04-03T00:35:40.897 startup: Get:1 http://deb.debian.org/debian bullseye/main amd64 libpython3.9-minimal amd64 3.9.2-1 [801 kB]
2021-04-03T00:35:40.955 startup: Get:2 http://deb.debian.org/debian bullseye/main amd64 libexpat1 amd64 2.2.10-2 [96.9 kB]
...
2021-04-03T00:35:44.128 startup: Setting up libpython3.9-stdlib:amd64 (3.9.2-1) ...
2021-04-03T00:35:44.151 startup: Setting up libpython3-stdlib:amd64 (3.9.2-2) ...
2021-04-03T00:35:44.172 startup: Setting up python3.9 (3.9.2-1) ...
2021-04-03T00:35:44.603 startup: Setting up python3 (3.9.2-2) ...
2021-04-03T00:35:44.751 startup: Setting up lsb-release (11.1.0) ...
2021-04-03T00:35:45.066 Listening on :9654 at /command
```

And the output resembles valid metrics
```metrics
# HELP command_duration_seconds duration in seconds that the command execution took
# TYPE command_duration_seconds gauge
command_duration_seconds{command="date"} 0.00840434
command_duration_seconds{command="os-release"} 1.424065033
# HELP command_success denotes whether the command ran successfully and exited success
# TYPE command_success gauge
command_success{command="date"} 1
command_success{command="os-release"} 1
# HELP date gets now()
# TYPE date gauge
date 1.6174102253199532e+09
# HELP os_release os release information
# TYPE os_release gauge
os_release{codename="bullseye",description="Debian GNU/Linux bullseye/sid",id="Debian",release="testing"} 1
```
