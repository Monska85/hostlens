# CI execution evidence

Local implementation validation passed. Review round 2 is clean; hosted candidate validation and timing comparisons are pending. No CI correction round has been consumed.

## Baseline

Three current-scope baseline samples run all ten matrix cases. The earlier six-case run is excluded from comparisons. Elapsed time includes scheduling; summed job execution is not billed cost. Job queue delay below measures job creation to start and excludes dependency wait before creation.

| Run                                                                          | Commit                                     | Elapsed seconds | Job minutes | Queue seconds (sum) |
| ---------------------------------------------------------------------------- | ------------------------------------------ | --------------- | ----------- | ------------------- |
| [34653976748](https://github.com/Monska85/hostlens/actions/runs/34653976748) | `b8f17be6cd991ed8021bf6a89b35c5641d1be68d` | 312             | 11.80       | 478                 |
| [34654507556](https://github.com/Monska85/hostlens/actions/runs/34654507556) | `bd460f0da21f6dab35328dbd8f714087609d91d8` | 280             | 12.42       | 542                 |
| [34656408216](https://github.com/Monska85/hostlens/actions/runs/34656408216) | `bd460f0da21f6dab35328dbd8f714087609d91d8` | 313             | 13.13       | 579                 |

## Maintained coverage

All checks retain locked tool setup, formatting, Ruff, ShellCheck, actionlint, vulnerability scanning, module verification, forced Go race tests and coverage, vet, Linux builds, portable package cross-builds, and Python release/matrix/developer regressions. The build job produces one amd64/arm64 archive pair; every case verifies and executes those bytes. The aggregate gate requires all jobs to succeed.

Cases: `debian-amd64`, `ubuntu-amd64`, `arch-amd64`, `debian-arm64`, `systemd-restricted`, `systemd-standard`, `audit-postgres`, `audit-nginx`, `audit-apache`, `audit-mysql`. Debian arm64 uses `ubuntu-24.04-arm`; every other job uses `ubuntu-24.04`. The matrix cap remains three pending measurement.

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
