# CI execution evidence

Local validation and all six corrected-revision hosted measurements passed. Five panel rounds are complete with no unresolved findings. One CI correction fixed restored compiler-cache ownership.

## Baseline

Three current-scope baseline samples run all ten matrix cases. The earlier six-case run is excluded from comparisons. Elapsed time includes scheduling; summed job execution is not billed cost. Job queue delay below measures job creation to start and excludes dependency wait before creation.

| Run                                                                          | Commit                                     | Elapsed seconds | Job minutes | Queue seconds (sum) |
| ---------------------------------------------------------------------------- | ------------------------------------------ | --------------- | ----------- | ------------------- |
| [34653976748](https://github.com/Monska85/hostlens/actions/runs/34653976748) | `b8f17be6cd991ed8021bf6a89b35c5641d1be68d` | 312             | 11.80       | 478                 |
| [34654507556](https://github.com/Monska85/hostlens/actions/runs/34654507556) | `bd460f0da21f6dab35328dbd8f714087609d91d8` | 280             | 12.42       | 542                 |
| [34656408216](https://github.com/Monska85/hostlens/actions/runs/34656408216) | `bd460f0da21f6dab35328dbd8f714087609d91d8` | 313             | 13.13       | 579                 |

## Maintained coverage

All checks retain locked tool setup, formatting, Ruff, ShellCheck, actionlint, vulnerability scanning, module verification, forced Go race tests and coverage, vet, Linux builds, portable package cross-builds, and Python release/matrix/developer regressions. The build job produces one amd64/arm64 archive pair; every case verifies and executes those bytes. The aggregate gate requires all jobs to succeed.

Cases: `debian-amd64`, `ubuntu-amd64`, `arch-amd64`, `debian-arm64`, `systemd-restricted`, `systemd-standard`, `audit-postgres`, `audit-nginx`, `audit-apache`, `audit-mysql`. Debian arm64 uses `ubuntu-24.04-arm`; every other job uses `ubuntu-24.04`. The matrix cap remains three based on the measurements below.

## Baseline job and step detail

Durations are seconds from GitHub API timestamps. Zero-second skipped and bookkeeping steps are omitted.

### Run 34653976748

| Job / step                                                             | Duration | Queue | Runner           |
| ---------------------------------------------------------------------- | -------- | ----- | ---------------- |
| **Go and static checks**                                               | **302**  | 4     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15       |       |                  |
| Install validation tools                                               | 16       |       |                  |
| Prepare validation image                                               | 64       |       |                  |
| Dependency vulnerability scan                                          | 5        |       |                  |
| Shared local and CI checks                                             | 194      |       |                  |
| Retain Go coverage reports                                             | 1        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Build candidate**                                                    | **31**   | 4     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 13       |       |                  |
| Prepare GoReleaser                                                     | 2        |       |                  |
| Build candidate once                                                   | 10       |       |                  |
| Run actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a   | 2        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / audit-apache**                                           | **30**   | 86    | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare platform image                                                 | 7        |       |                  |
| Run selected container case                                            | 3        |       |                  |
| **Container / debian-amd64**                                           | **31**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16       |       |                  |
| Prepare platform image                                                 | 3        |       |                  |
| Run selected container case                                            | 6        |       |                  |
| **Container / audit-postgres**                                         | **43**   | 71    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 4        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16       |       |                  |
| Prepare platform image                                                 | 12       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / systemd-restricted**                                     | **41**   | 35    | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare systemd image                                                  | 13       |       |                  |
| Run selected container case                                            | 8        |       |                  |
| Complete job                                                           | 1        |       |                  |
| **Container / systemd-standard**                                       | **45**   | 39    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12       |       |                  |
| Prepare systemd image                                                  | 14       |       |                  |
| Run selected container case                                            | 9        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / audit-mysql**                                            | **52**   | 116   | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15       |       |                  |
| Prepare platform image                                                 | 19       |       |                  |
| Run selected container case                                            | 7        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Container / audit-nginx**                                            | **34**   | 78    | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12       |       |                  |
| Prepare platform image                                                 | 14       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Complete job                                                           | 1        |       |                  |
| **Container / ubuntu-amd64**                                           | **31**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15       |       |                  |
| Prepare platform image                                                 | 5        |       |                  |
| Run selected container case                                            | 4        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / arch-amd64**                                             | **35**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare platform image                                                 | 9        |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / debian-arm64**                                           | **30**   | 37    | ubuntu-24.04-arm |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 4        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 10       |       |                  |
| Prepare platform image                                                 | 4        |       |                  |
| Run selected container case                                            | 6        |       |                  |
| **CI gate**                                                            | **3**    | 2     | ubuntu-24.04     |

### Run 34654507556

| Job / step                                                             | Duration | Queue | Runner           |
| ---------------------------------------------------------------------- | -------- | ----- | ---------------- |
| **Build candidate**                                                    | **39**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16       |       |                  |
| Prepare GoReleaser                                                     | 2        |       |                  |
| Build candidate once                                                   | 10       |       |                  |
| Run actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a   | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Go and static checks**                                               | **269**  | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 21       |       |                  |
| Install validation tools                                               | 13       |       |                  |
| Prepare validation image                                               | 69       |       |                  |
| Dependency vulnerability scan                                          | 4        |       |                  |
| Shared local and CI checks                                             | 154      |       |                  |
| Retain Go coverage reports                                             | 1        |       |                  |
| **Container / ubuntu-amd64**                                           | **30**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 11       |       |                  |
| Prepare platform image                                                 | 4        |       |                  |
| Run selected container case                                            | 4        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Container / audit-apache**                                           | **28**   | 118   | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12       |       |                  |
| Prepare platform image                                                 | 6        |       |                  |
| Run selected container case                                            | 4        |       |                  |
| **Container / debian-amd64**                                           | **41**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 19       |       |                  |
| Prepare platform image                                                 | 6        |       |                  |
| Run selected container case                                            | 8        |       |                  |
| **Container / systemd-standard**                                       | **43**   | 48    | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15       |       |                  |
| Prepare systemd image                                                  | 13       |       |                  |
| Run selected container case                                            | 9        |       |                  |
| **Container / audit-nginx**                                            | **40**   | 93    | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 18       |       |                  |
| Prepare platform image                                                 | 8        |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / debian-arm64**                                           | **26**   | 36    | ubuntu-24.04-arm |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 13       |       |                  |
| Prepare platform image                                                 | 2        |       |                  |
| Run selected container case                                            | 5        |       |                  |
| **Container / arch-amd64**                                             | **44**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 3        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare platform image                                                 | 16       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Container / systemd-restricted**                                     | **78**   | 44    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare systemd image                                                  | 13       |       |                  |
| Run selected container case                                            | 8        |       |                  |
| **Container / audit-mysql**                                            | **53**   | 125   | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 18       |       |                  |
| Prepare platform image                                                 | 19       |       |                  |
| Run selected container case                                            | 6        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / audit-postgres**                                         | **50**   | 66    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 20       |       |                  |
| Prepare platform image                                                 | 17       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **CI gate**                                                            | **4**    | 2     | ubuntu-24.04     |

### Run 34656408216

| Job / step                                                             | Duration | Queue | Runner           |
| ---------------------------------------------------------------------- | -------- | ----- | ---------------- |
| **Build candidate**                                                    | **41**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 20       |       |                  |
| Prepare GoReleaser                                                     | 3        |       |                  |
| Build candidate once                                                   | 8        |       |                  |
| Run actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a   | 2        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Go and static checks**                                               | **301**  | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12       |       |                  |
| Install validation tools                                               | 15       |       |                  |
| Prepare validation image                                               | 61       |       |                  |
| Dependency vulnerability scan                                          | 4        |       |                  |
| Shared local and CI checks                                             | 200      |       |                  |
| Retain Go coverage reports                                             | 2        |       |                  |
| **Container / debian-amd64**                                           | **42**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 20       |       |                  |
| Prepare platform image                                                 | 6        |       |                  |
| Run selected container case                                            | 8        |       |                  |
| **Container / ubuntu-amd64**                                           | **45**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 21       |       |                  |
| Prepare platform image                                                 | 5        |       |                  |
| Run selected container case                                            | 9        |       |                  |
| **Container / systemd-restricted**                                     | **45**   | 50    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16       |       |                  |
| Prepare systemd image                                                  | 14       |       |                  |
| Run selected container case                                            | 8        |       |                  |
| **Container / systemd-standard**                                       | **57**   | 51    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 21       |       |                  |
| Prepare systemd image                                                  | 17       |       |                  |
| Run selected container case                                            | 10       |       |                  |
| **Container / audit-apache**                                           | **44**   | 111   | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 20       |       |                  |
| Prepare platform image                                                 | 12       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| **Container / debian-arm64**                                           | **26**   | 49    | ubuntu-24.04-arm |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 13       |       |                  |
| Prepare platform image                                                 | 2        |       |                  |
| Run selected container case                                            | 5        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **Container / audit-nginx**                                            | **39**   | 99    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 17       |       |                  |
| Prepare platform image                                                 | 8        |       |                  |
| Run selected container case                                            | 3        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / audit-postgres**                                         | **49**   | 78    | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14       |       |                  |
| Prepare platform image                                                 | 24       |       |                  |
| Run selected container case                                            | 3        |       |                  |
| **Container / arch-amd64**                                             | **47**   | 2     | ubuntu-24.04     |
| Set up job                                                             | 2        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 19       |       |                  |
| Prepare platform image                                                 | 16       |       |                  |
| Run selected container case                                            | 2        |       |                  |
| Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1        |       |                  |
| **Container / audit-mysql**                                            | **48**   | 129   | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |
| Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1        |       |                  |
| Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1        |       |                  |
| Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16       |       |                  |
| Prepare acceptance helper dependencies                                 | 1        |       |                  |
| Prepare platform image                                                 | 18       |       |                  |
| Run selected container case                                            | 6        |       |                  |
| Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1        |       |                  |
| **CI gate**                                                            | **4**    | 2     | ubuntu-24.04     |
| Set up job                                                             | 1        |       |                  |

## Local acceptance and review

The complete disposable-container race/coverage/vet/build and Python suites passed after review fixes, with 78.2% internal statement coverage. A current govulncheck scan found no vulnerabilities. Both candidate archives passed verification; all ten matrix cases passed, including emulated ARM64 and both native systemd privilege modes. Formatting, Ruff, ShellCheck, actionlint and GoReleaser configuration checks passed. The validation image used Go 1.27.0 on Linux amd64; local containers share the host kernel.

Panel round 1 used CI/CD, security, specification, adversarial, maintainability, performance and idiomatic review perspectives. Two medium findings were verified and fixed: enforce a private compiler-cache parent, and add a regression for the changed platform launcher's cancellation behavior. Round 2 rechecked the fixes and reported no actionable findings. No findings were dismissed.

A disposable source copy added `TestWarmCacheStillFails` after cache warmup. The test executed and reported its deliberate failure; the launcher returned failure. The copy was never committed. A real `audit-mysql` acceptance run was interrupted after its named Docker container started: the dispatcher returned 130 and the container disappeared. Local image preparation also passed through the unchanged Make entry points without hosted credentials.

The final local warm-cache suite reported 78.1% statement coverage (the preceding run reported 78.2%; timing-dependent execution paths vary). All 15 Python regression tests passed, including the new private-cache and platform-cancellation checks.

## Hosted cancellation and isolation

The temporary `ci/isolation-probe` branch invoked the actual reusable CI with `release_validation: true`, without a delivery job. After its second push, ordinary run [34656970693](https://github.com/Monska85/hostlens/actions/runs/34656970693) was cancelled; replacement [34656992934](https://github.com/Monska85/hostlens/actions/runs/34656992934) passed. Both reusable runs, [34656970886](https://github.com/Monska85/hostlens/actions/runs/34656970886) and [34656993433](https://github.com/Monska85/hostlens/actions/runs/34656993433), passed. The unrelated candidate branch's [seed run](https://github.com/Monska85/hostlens/actions/runs/34656837717) and [uncached run](https://github.com/Monska85/hostlens/actions/runs/34656900422) also passed.

This directly exercises ordinary supersession, unrelated-branch independence and reusable release-mode isolation. No release tag or release was published; tag scoping and the separate production caller group were also checked in the reviewed workflow expressions. All successful runs retained native hosted ARM64, both systemd modes and every application case.

The complete local suite also passed with `HOSTLENS_BUILD_CACHE` unset. Caches are optional in both environments.

## CI correction round 1

The predecessor revision `fbb23e6` passed all three uncached samples, but its first [warm run](https://github.com/Monska85/hostlens/actions/runs/34658157957) failed the vulnerability scan. GitHub restored compiler files as the runner UID; the capability-dropped container could not overwrite a Go cache entry. The aggregate gate failed correctly.

The fix normalizes caller-owned cache entry permissions beneath the verified private parent. A regression covers restored restrictive directory/file modes. A host-owned copy of the populated compiler cache then passed the real scanner and complete container coverage suite. Panel round 3 reviewed the permission boundary and implementation and found no actionable issues. Final measurements restart on the corrected revision; predecessor timings are not presented as its performance results.

## Final portability review

Round 4 replaced the cache guard's GNU `stat` invocation with the already-required Python standard library, preserving the owner/mode check for macOS development clients using a Linux Docker engine. The fixture now supplies an unusable `stat` command, so reintroducing that dependency fails the regression. The full container coverage suite and scanner passed; the panel reported no actionable findings. Native macOS execution was not performed.

Timing samples remain on fixed revision `bcfab568f24f9a97a080379a29386ca13d3c3116`. The final follow-up replaces the host ownership metadata check, guards its portability and makes the cancellation fixture PID marker atomic. It does not alter product code, cache keys, image preparation or scheduling. The final published revision receives its own complete CI validation. Reported benchmark numbers identify their measured revision rather than presenting a later documentation/archive commit as measured.

## Candidate timing comparison

All candidate samples use revision `bcfab568f24f9a97a080379a29386ca13d3c3116` and the same thirteen-job validation scope. Cold samples explicitly disable all hosted caches, including pre-existing setup-go reuse. Warm samples enable caches after the successful seed run on the same ref. Baseline setup-go caches were enabled, so cold-versus-baseline is not an estimate of the new caches' effect.

| Condition | Samples | Mean elapsed seconds (range) | Mean job minutes (range) | Mean summed queue seconds |
| --------- | ------- | ---------------------------- | ------------------------ | ------------------------- |
| baseline  | 3       | 301.67 (280.00 to 313.00)    | 12.45 (11.80 to 13.13)   | 533                       |
| cold      | 3       | 365.00 (341.00 to 411.00)    | 17.13 (16.78 to 17.58)   | 743                       |
| warm      | 3       | 228.67 (226.00 to 232.00)    | 10.21 (9.42 to 10.63)    | 563                       |

| Candidate run                                                                | Condition | Elapsed seconds | Job minutes | Queue sum seconds |
| ---------------------------------------------------------------------------- | --------- | --------------- | ----------- | ----------------- |
| [34659303754](https://github.com/Monska85/hostlens/actions/runs/34659303754) | cold      | 411             | 17.58       | 786               |
| [34659739714](https://github.com/Monska85/hostlens/actions/runs/34659739714) | cold      | 341             | 17.02       | 717               |
| [34660083333](https://github.com/Monska85/hostlens/actions/runs/34660083333) | cold      | 343             | 16.78       | 726               |
| [34658547946](https://github.com/Monska85/hostlens/actions/runs/34658547946) | warm      | 232             | 10.58       | 570               |
| [34658813645](https://github.com/Monska85/hostlens/actions/runs/34658813645) | warm      | 226             | 9.42        | 511               |
| [34659057852](https://github.com/Monska85/hostlens/actions/runs/34659057852) | warm      | 228             | 10.63       | 609               |

### Mean job durations

Seconds from job start to completion, including setup and cleanup. ARM64 runs on `ubuntu-24.04-arm`; other jobs run on `ubuntu-24.04`.

| Job                            | Baseline | Cold  | Warm  |
| ------------------------------ | -------- | ----- | ----- |
| Build candidate                | 37.0     | 79.3  | 38.0  |
| CI gate                        | 3.7      | 3.3   | 3.0   |
| Container / arch-amd64         | 42.0     | 56.7  | 49.3  |
| Container / audit-apache       | 34.0     | 54.7  | 49.3  |
| Container / audit-mysql        | 51.0     | 73.7  | 53.3  |
| Container / audit-nginx        | 37.7     | 55.0  | 35.3  |
| Container / audit-postgres     | 47.3     | 61.0  | 50.0  |
| Container / debian-amd64       | 38.0     | 47.7  | 36.0  |
| Container / debian-arm64       | 27.3     | 40.7  | 28.0  |
| Container / systemd-restricted | 54.7     | 80.0  | 61.0  |
| Container / systemd-standard   | 48.3     | 82.0  | 58.3  |
| Container / ubuntu-amd64       | 35.3     | 51.0  | 36.0  |
| Go and static checks           | 290.7    | 342.7 | 115.0 |

### Candidate preparation and checks

Per-step seconds, including cache transfer and builder cleanup rather than attributing the entire job difference to compilation.

| Run         | Job                            | Step                                                                   | Seconds |
| ----------- | ------------------------------ | ---------------------------------------------------------------------- | ------- |
| 34659303754 | Go and static checks           | Set up job                                                             | 2       |
| 34659303754 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659303754 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 11      |
| 34659303754 | Go and static checks           | Install validation tools                                               | 29      |
| 34659303754 | Go and static checks           | Prepare Buildx                                                         | 7       |
| 34659303754 | Go and static checks           | Prepare validation image                                               | 87      |
| 34659303754 | Go and static checks           | Dependency vulnerability scan                                          | 4       |
| 34659303754 | Go and static checks           | Shared local and CI checks                                             | 212     |
| 34659303754 | Go and static checks           | Retain Go coverage reports                                             | 2       |
| 34659303754 | Go and static checks           | Post Prepare validation image                                          | 2       |
| 34659303754 | Go and static checks           | Post Prepare Buildx                                                    | 3       |
| 34659303754 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659303754 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34659303754 | Go and static checks           | Complete job                                                           | 0       |
| 34659303754 | Container / systemd-standard   | Set up job                                                             | 1       |
| 34659303754 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659303754 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1       |
| 34659303754 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 13      |
| 34659303754 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 1       |
| 34659303754 | Container / systemd-standard   | Prepare Buildx                                                         | 8       |
| 34659303754 | Container / systemd-standard   | Prepare systemd image                                                  | 12      |
| 34659303754 | Container / systemd-standard   | Run selected container case                                            | 31      |
| 34659303754 | Container / systemd-standard   | Post Prepare systemd image                                             | 1       |
| 34659303754 | Container / systemd-standard   | Post Prepare Buildx                                                    | 1       |
| 34659303754 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659303754 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34659303754 | Container / systemd-standard   | Complete job                                                           | 0       |
| 34659303754 | Container / systemd-restricted | Set up job                                                             | 4       |
| 34659303754 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659303754 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34659303754 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12      |
| 34659303754 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 1       |
| 34659303754 | Container / systemd-restricted | Prepare Buildx                                                         | 6       |
| 34659303754 | Container / systemd-restricted | Prepare systemd image                                                  | 16      |
| 34659303754 | Container / systemd-restricted | Run selected container case                                            | 30      |
| 34659303754 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34659303754 | Container / systemd-restricted | Post Prepare Buildx                                                    | 0       |
| 34659303754 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659303754 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34659303754 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34659739714 | Go and static checks           | Set up job                                                             | 6       |
| 34659739714 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2       |
| 34659739714 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 13      |
| 34659739714 | Go and static checks           | Install validation tools                                               | 27      |
| 34659739714 | Go and static checks           | Prepare Buildx                                                         | 13      |
| 34659739714 | Go and static checks           | Prepare validation image                                               | 95      |
| 34659739714 | Go and static checks           | Dependency vulnerability scan                                          | 4       |
| 34659739714 | Go and static checks           | Shared local and CI checks                                             | 161     |
| 34659739714 | Go and static checks           | Retain Go coverage reports                                             | 1       |
| 34659739714 | Go and static checks           | Post Prepare validation image                                          | 2       |
| 34659739714 | Go and static checks           | Post Prepare Buildx                                                    | 2       |
| 34659739714 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659739714 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34659739714 | Go and static checks           | Complete job                                                           | 0       |
| 34659739714 | Container / systemd-standard   | Set up job                                                             | 4       |
| 34659739714 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2       |
| 34659739714 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34659739714 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 18      |
| 34659739714 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 1       |
| 34659739714 | Container / systemd-standard   | Prepare Buildx                                                         | 10      |
| 34659739714 | Container / systemd-standard   | Prepare systemd image                                                  | 19      |
| 34659739714 | Container / systemd-standard   | Run selected container case                                            | 25      |
| 34659739714 | Container / systemd-standard   | Post Prepare systemd image                                             | 2       |
| 34659739714 | Container / systemd-standard   | Post Prepare Buildx                                                    | 1       |
| 34659739714 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659739714 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34659739714 | Container / systemd-standard   | Complete job                                                           | 0       |
| 34659739714 | Container / systemd-restricted | Set up job                                                             | 3       |
| 34659739714 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659739714 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34659739714 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 10      |
| 34659739714 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 1       |
| 34659739714 | Container / systemd-restricted | Prepare Buildx                                                         | 9       |
| 34659739714 | Container / systemd-restricted | Prepare systemd image                                                  | 16      |
| 34659739714 | Container / systemd-restricted | Run selected container case                                            | 29      |
| 34659739714 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34659739714 | Container / systemd-restricted | Post Prepare Buildx                                                    | 0       |
| 34659739714 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1       |
| 34659739714 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34659739714 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34660083333 | Go and static checks           | Set up job                                                             | 6       |
| 34660083333 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2       |
| 34660083333 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 17      |
| 34660083333 | Go and static checks           | Install validation tools                                               | 27      |
| 34660083333 | Go and static checks           | Prepare Buildx                                                         | 11      |
| 34660083333 | Go and static checks           | Prepare validation image                                               | 89      |
| 34660083333 | Go and static checks           | Dependency vulnerability scan                                          | 4       |
| 34660083333 | Go and static checks           | Shared local and CI checks                                             | 169     |
| 34660083333 | Go and static checks           | Retain Go coverage reports                                             | 1       |
| 34660083333 | Go and static checks           | Post Prepare validation image                                          | 2       |
| 34660083333 | Go and static checks           | Post Prepare Buildx                                                    | 1       |
| 34660083333 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1       |
| 34660083333 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34660083333 | Go and static checks           | Complete job                                                           | 0       |
| 34660083333 | Container / systemd-restricted | Set up job                                                             | 4       |
| 34660083333 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34660083333 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34660083333 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12      |
| 34660083333 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 1       |
| 34660083333 | Container / systemd-restricted | Prepare Buildx                                                         | 8       |
| 34660083333 | Container / systemd-restricted | Prepare systemd image                                                  | 16      |
| 34660083333 | Container / systemd-restricted | Run selected container case                                            | 29      |
| 34660083333 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34660083333 | Container / systemd-restricted | Post Prepare Buildx                                                    | 1       |
| 34660083333 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34660083333 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34660083333 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34660083333 | Container / systemd-standard   | Set up job                                                             | 5       |
| 34660083333 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34660083333 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2       |
| 34660083333 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 12      |
| 34660083333 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 0       |
| 34660083333 | Container / systemd-standard   | Prepare Buildx                                                         | 15      |
| 34660083333 | Container / systemd-standard   | Prepare systemd image                                                  | 15      |
| 34660083333 | Container / systemd-standard   | Run selected container case                                            | 28      |
| 34660083333 | Container / systemd-standard   | Post Prepare systemd image                                             | 2       |
| 34660083333 | Container / systemd-standard   | Post Prepare Buildx                                                    | 0       |
| 34660083333 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34660083333 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34660083333 | Container / systemd-standard   | Complete job                                                           | 0       |
| 34658547946 | Go and static checks           | Set up job                                                             | 2       |
| 34658547946 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34658547946 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14      |
| 34658547946 | Go and static checks           | Install validation tools                                               | 16      |
| 34658547946 | Go and static checks           | Prepare Buildx                                                         | 7       |
| 34658547946 | Go and static checks           | Prepare validation image                                               | 20      |
| 34658547946 | Go and static checks           | Restore container compiler cache                                       | 3       |
| 34658547946 | Go and static checks           | Select optional compiler cache                                         | 0       |
| 34658547946 | Go and static checks           | Dependency vulnerability scan                                          | 7       |
| 34658547946 | Go and static checks           | Shared local and CI checks                                             | 29      |
| 34658547946 | Go and static checks           | Retain Go coverage reports                                             | 1       |
| 34658547946 | Go and static checks           | Post Restore container compiler cache                                  | 5       |
| 34658547946 | Go and static checks           | Post Prepare validation image                                          | 1       |
| 34658547946 | Go and static checks           | Post Prepare Buildx                                                    | 1       |
| 34658547946 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34658547946 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34658547946 | Go and static checks           | Complete job                                                           | 1       |
| 34658547946 | Container / systemd-restricted | Set up job                                                             | 2       |
| 34658547946 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2       |
| 34658547946 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34658547946 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 18      |
| 34658547946 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 0       |
| 34658547946 | Container / systemd-restricted | Prepare Buildx                                                         | 11      |
| 34658547946 | Container / systemd-restricted | Prepare systemd image                                                  | 9       |
| 34658547946 | Container / systemd-restricted | Run selected container case                                            | 9       |
| 34658547946 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34658547946 | Container / systemd-restricted | Post Prepare Buildx                                                    | 0       |
| 34658547946 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34658547946 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34658547946 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34658547946 | Container / systemd-standard   | Set up job                                                             | 3       |
| 34658547946 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34658547946 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1       |
| 34658547946 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14      |
| 34658547946 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 0       |
| 34658547946 | Container / systemd-standard   | Prepare Buildx                                                         | 15      |
| 34658547946 | Container / systemd-standard   | Prepare systemd image                                                  | 7       |
| 34658547946 | Container / systemd-standard   | Run selected container case                                            | 10      |
| 34658547946 | Container / systemd-standard   | Post Prepare systemd image                                             | 1       |
| 34658547946 | Container / systemd-standard   | Post Prepare Buildx                                                    | 0       |
| 34658547946 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1       |
| 34658547946 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34658547946 | Container / systemd-standard   | Complete job                                                           | 0       |
| 34658813645 | Go and static checks           | Set up job                                                             | 2       |
| 34658813645 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34658813645 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15      |
| 34658813645 | Go and static checks           | Install validation tools                                               | 15      |
| 34658813645 | Go and static checks           | Prepare Buildx                                                         | 8       |
| 34658813645 | Go and static checks           | Prepare validation image                                               | 20      |
| 34658813645 | Go and static checks           | Restore container compiler cache                                       | 4       |
| 34658813645 | Go and static checks           | Select optional compiler cache                                         | 0       |
| 34658813645 | Go and static checks           | Dependency vulnerability scan                                          | 6       |
| 34658813645 | Go and static checks           | Shared local and CI checks                                             | 31      |
| 34658813645 | Go and static checks           | Retain Go coverage reports                                             | 1       |
| 34658813645 | Go and static checks           | Post Restore container compiler cache                                  | 0       |
| 34658813645 | Go and static checks           | Post Prepare validation image                                          | 1       |
| 34658813645 | Go and static checks           | Post Prepare Buildx                                                    | 1       |
| 34658813645 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34658813645 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34658813645 | Go and static checks           | Complete job                                                           | 0       |
| 34658813645 | Container / systemd-standard   | Set up job                                                             | 4       |
| 34658813645 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 2       |
| 34658813645 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 2       |
| 34658813645 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 15      |
| 34658813645 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 0       |
| 34658813645 | Container / systemd-standard   | Prepare Buildx                                                         | 15      |
| 34658813645 | Container / systemd-standard   | Prepare systemd image                                                  | 8       |
| 34658813645 | Container / systemd-standard   | Run selected container case                                            | 10      |
| 34658813645 | Container / systemd-standard   | Post Prepare systemd image                                             | 1       |
| 34658813645 | Container / systemd-standard   | Post Prepare Buildx                                                    | 1       |
| 34658813645 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34658813645 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34658813645 | Container / systemd-standard   | Complete job                                                           | 0       |
| 34658813645 | Container / systemd-restricted | Set up job                                                             | 2       |
| 34658813645 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34658813645 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 1       |
| 34658813645 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 14      |
| 34658813645 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 0       |
| 34658813645 | Container / systemd-restricted | Prepare Buildx                                                         | 9       |
| 34658813645 | Container / systemd-restricted | Prepare systemd image                                                  | 5       |
| 34658813645 | Container / systemd-restricted | Run selected container case                                            | 8       |
| 34658813645 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34658813645 | Container / systemd-restricted | Post Prepare Buildx                                                    | 0       |
| 34658813645 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34658813645 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34658813645 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34659057852 | Go and static checks           | Set up job                                                             | 2       |
| 34659057852 | Go and static checks           | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659057852 | Go and static checks           | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 17      |
| 34659057852 | Go and static checks           | Install validation tools                                               | 18      |
| 34659057852 | Go and static checks           | Prepare Buildx                                                         | 14      |
| 34659057852 | Go and static checks           | Prepare validation image                                               | 26      |
| 34659057852 | Go and static checks           | Restore container compiler cache                                       | 3       |
| 34659057852 | Go and static checks           | Select optional compiler cache                                         | 0       |
| 34659057852 | Go and static checks           | Dependency vulnerability scan                                          | 12      |
| 34659057852 | Go and static checks           | Shared local and CI checks                                             | 27      |
| 34659057852 | Go and static checks           | Retain Go coverage reports                                             | 0       |
| 34659057852 | Go and static checks           | Post Restore container compiler cache                                  | 0       |
| 34659057852 | Go and static checks           | Post Prepare validation image                                          | 2       |
| 34659057852 | Go and static checks           | Post Prepare Buildx                                                    | 0       |
| 34659057852 | Go and static checks           | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 1       |
| 34659057852 | Go and static checks           | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34659057852 | Go and static checks           | Complete job                                                           | 0       |
| 34659057852 | Container / systemd-restricted | Set up job                                                             | 2       |
| 34659057852 | Container / systemd-restricted | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659057852 | Container / systemd-restricted | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 3       |
| 34659057852 | Container / systemd-restricted | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16      |
| 34659057852 | Container / systemd-restricted | Prepare acceptance helper dependencies                                 | 0       |
| 34659057852 | Container / systemd-restricted | Prepare Buildx                                                         | 9       |
| 34659057852 | Container / systemd-restricted | Prepare systemd image                                                  | 10      |
| 34659057852 | Container / systemd-restricted | Run selected container case                                            | 8       |
| 34659057852 | Container / systemd-restricted | Post Prepare systemd image                                             | 2       |
| 34659057852 | Container / systemd-restricted | Post Prepare Buildx                                                    | 0       |
| 34659057852 | Container / systemd-restricted | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659057852 | Container / systemd-restricted | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 1       |
| 34659057852 | Container / systemd-restricted | Complete job                                                           | 0       |
| 34659057852 | Container / systemd-standard   | Set up job                                                             | 2       |
| 34659057852 | Container / systemd-standard   | Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1          | 1       |
| 34659057852 | Container / systemd-standard   | Run actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c | 4       |
| 34659057852 | Container / systemd-standard   | Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e          | 16      |
| 34659057852 | Container / systemd-standard   | Prepare acceptance helper dependencies                                 | 0       |
| 34659057852 | Container / systemd-standard   | Prepare Buildx                                                         | 10      |
| 34659057852 | Container / systemd-standard   | Prepare systemd image                                                  | 9       |
| 34659057852 | Container / systemd-standard   | Run selected container case                                            | 9       |
| 34659057852 | Container / systemd-standard   | Post Prepare systemd image                                             | 2       |
| 34659057852 | Container / systemd-standard   | Post Prepare Buildx                                                    | 1       |
| 34659057852 | Container / systemd-standard   | Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e     | 0       |
| 34659057852 | Container / systemd-standard   | Post Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1     | 0       |
| 34659057852 | Container / systemd-standard   | Complete job                                                           | 0       |

### Image and cache evidence

Image configuration digests reported by Buildx below identify actual validation and systemd outputs. Full action logs linked above also retain base/fixture image digests and cache keys. Compiler storage is keyed again by this local image identity.

| Run         | Image configuration digests                                                                                                                                                                                                     |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 34659303754 | `sha256:044c2ec6643767ae1a152d267fdc880a2e6673a4ac7e92f51aa2e7a5d6eab0ed`, `sha256:0902a477566b56c36809ae7448116774108fbc71ab98b751ef74b79acbfb3834`, `sha256:d24e85050e00b6579fc5f6a856fcb78c14f83680fdf63aef5cf197b9b5f45ea7` |
| 34659739714 | `sha256:234a17ddf619f784a0476d17130125db1bba4d74c4a1cb05d803e3585cb3aecc`, `sha256:29fbff08881a5b1f67e1ec3679b4e4e7b4f302c30c92f1671d94e462da4f3f76`, `sha256:7348f8a3a1ced4eaea8d7c06566d9812c54b626972a08b31f6d8679f4fd42ba6` |
| 34660083333 | `sha256:19bbc3eb1970cd33bf1491af9220852cbc3cdf730e9fc4d834abce95f69a6021`, `sha256:2eafdfce1c7141c3e238f58d9de3f0c84a4f4c400b10e3ca5ce0c320bc918c62`, `sha256:dd1175c200a24dcf4d18c92c84739139489cc39d6b41f3ed06bb27c20951851e` |
| 34658547946 | `sha256:832996c8de78ab477aa996f46007f6426e5dfe137814ed2176255c656a9d8443`, `sha256:ad758d336d6cab032a58c1256ad8a5c4c3b2c68ba721f4149c5898a499c50f46`                                                                            |
| 34658813645 | `sha256:832996c8de78ab477aa996f46007f6426e5dfe137814ed2176255c656a9d8443`, `sha256:ad758d336d6cab032a58c1256ad8a5c4c3b2c68ba721f4149c5898a499c50f46`                                                                            |
| 34659057852 | `sha256:832996c8de78ab477aa996f46007f6426e5dfe137814ed2176255c656a9d8443`, `sha256:ad758d336d6cab032a58c1256ad8a5c4c3b2c68ba721f4149c5898a499c50f46`                                                                            |

Hosted runner and queue variance are not controlled. Three samples per condition estimate this workload only. Mutable representative application images and third-party package mirrors can also affect timings. Summed job execution is neither wall time nor billed cost.

## Retained tuning choices

The warm sample mean improved by 24.2% in elapsed time and 18.0% in summed job execution versus the baseline. The checks job mean fell from 290.7 to 115.0 seconds. This is an observed result for these runner environments and this workload, not a guarantee for future runs. Candidate validation preserved every existing check and added two focused harness regressions.

The matrix cap remains three. Warm summed queue delay averaged 563 seconds versus 533 baseline seconds, while end-to-end completion still improved. Higher concurrency might shorten the new matrix critical path but would raise simultaneous runner demand; there is no controlled measurement supporting that trade-off, so the existing limit is retained. No helper or setup step was removed without evidence of redundancy.

Systemd job means increased from 54.7/48.3 seconds to 61.0/58.3 seconds for restricted/standard modes. No systemd speed improvement is claimed. Its standard layer cache is retained for reproducible fixture reuse under cache hits: both privilege modes used configuration digest `sha256:832996c8de78ab477aa996f46007f6426e5dfe137814ed2176255c656a9d8443` in all warm samples. Cold independent builds can still differ. The explicit uncached path remains available to refresh and validate prerequisites.

The compiler cache was about 191 MiB. Hosted cache metadata placed its entries under the expected branch refs with the `branch` namespace; no cross-trust restore prefix was added. The final capability-dropped container suite passed with restored host-owned cache entries and with caching disabled. Local runtime evidence remains container-scoped, with emulated ARM64 distinguished from native hosted ARM64.

## Final review

The fifth panel round found no actionable issues across CI, security, specification, adversarial behavior, maintainability, performance and idiomatic implementation. Reviewers independently checked timing arithmetic, cache boundaries, cancellation assertions and the distinction between measured and final revisions. The final portable cache guard and atomic fixture marker passed the complete container test suite. Publication requires successful CI on the exact final main commit.
