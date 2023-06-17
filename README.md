# Sablier

[![GitHub license](https://img.shields.io/github/license/acouvreur/sablier.svg)](https://github.com/acouvreur/sablier/blob/master/LICENSE)
[![GitHub contributors](https://img.shields.io/github/contributors/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/graphs/contributors/)
[![GitHub issues](https://img.shields.io/github/issues/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/issues/)
[![GitHub pull-requests](https://img.shields.io/github/issues-pr/acouvreur/sablier.svg)](https://GitHub.com/acouvreur/sablier/pulls/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](http://makeapullrequest.com)

[![GoDoc](https://godoc.org/github.com/acouvreur/sablier?status.svg)](http://godoc.org/github.com/acouvreur/sablier)
![Latest Build](https://img.shields.io/github/actions/workflow/status/acouvreur/sablier/build.yml?style=flat-square&branch=main)
![Go Report](https://goreportcard.com/badge/github.com/acouvreur/sablier?style=flat-square)
![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/sablier?style=flat-square)
![Latest Release](https://img.shields.io/github/v/release/acouvreur/sablier?style=flat-square&sort=semver)
![Latest PreRelease](https://img.shields.io/github/v/release/acouvreur/sablier?style=flat-square&include_prereleases&sort=semver)

An free and open-source software to bring workloads on demand to your infrastructure.

Either because you don't want to overload your raspberry pi or because your QA environment gets used only once a week and wastes resources by keeping your workloads up and running, Sablier is a project that might interest you.

## 🎯 Features

- [Supports the following providers](https://acouvreur.github.io/sablier/#/providers/overview)
  - Docker
  - Docker Swarm
  - Kubernetes
- [Supports multiple reverse proxies](https://acouvreur.github.io/sablier/#/plugins/overview)
  - Nginx
  - Traefik
  - Caddy
- Scale up your workload automatically upon the first request
  - [with a themable waiting page]()
  - [with a hanging request (hang until service is up)]()
- Scale your workload to zero automatically after a period of inactivity

## 📝 Documentation

[See the documentation here](https://acouvreur.github.io/sablier/#/)

## Contributors

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://www.alexiscouvreur.fr/"><img src="https://avatars.githubusercontent.com/u/22034450?v=4?s=100" width="100px;" alt="Alexis Couvreur"/><br /><sub><b>Alexis Couvreur</b></sub></a><br /><a href="#question-acouvreur" title="Answering Questions">💬</a> <a href="https://github.com/acouvreur/sablier/issues?q=author%3Aacouvreur" title="Bug reports">🐛</a> <a href="https://github.com/acouvreur/sablier/commits?author=acouvreur" title="Code">💻</a> <a href="https://github.com/acouvreur/sablier/commits?author=acouvreur" title="Documentation">📖</a> <a href="#example-acouvreur" title="Examples">💡</a> <a href="#ideas-acouvreur" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/mschneider82"><img src="https://avatars.githubusercontent.com/u/8426497?v=4?s=100" width="100px;" alt="Matthias Schneider"/><br /><sub><b>Matthias Schneider</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=mschneider82" title="Code">💻</a> <a href="https://github.com/acouvreur/sablier/commits?author=mschneider82" title="Documentation">📖</a> <a href="https://github.com/acouvreur/sablier/pulls?q=is%3Apr+reviewed-by%3Amschneider82" title="Reviewed Pull Requests">👀</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Thyvador"><img src="https://avatars.githubusercontent.com/u/20644197?v=4?s=100" width="100px;" alt="Alexandre HILTCHER"/><br /><sub><b>Alexandre HILTCHER</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=Thyvador" title="Code">💻</a> <a href="#ideas-Thyvador" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/tandy-1000"><img src="https://avatars.githubusercontent.com/u/24867509?v=4?s=100" width="100px;" alt="tandy1000"/><br /><sub><b>tandy1000</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=tandy-1000" title="Documentation">📖</a> <a href="#ideas-tandy-1000" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Sam-R"><img src="https://avatars.githubusercontent.com/u/4183297?v=4?s=100" width="100px;" alt="Sam R."/><br /><sub><b>Sam R.</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=Sam-R" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Nastaliss"><img src="https://avatars.githubusercontent.com/u/46960549?v=4?s=100" width="100px;" alt="Stanislas Bruhière"/><br /><sub><b>Stanislas Bruhière</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=Nastaliss" title="Code">💻</a> <a href="#ideas-Nastaliss" title="Ideas, Planning, & Feedback">🤔</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/sourgrasses"><img src="https://avatars.githubusercontent.com/u/12515536?v=4?s=100" width="100px;" alt="Jenn Wheeler"/><br /><sub><b>Jenn Wheeler</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=sourgrasses" title="Code">💻</a> <a href="#ideas-sourgrasses" title="Ideas, Planning, & Feedback">🤔</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/romdum"><img src="https://avatars.githubusercontent.com/u/19633997?v=4?s=100" width="100px;" alt="Romain Duminil"/><br /><sub><b>Romain Duminil</b></sub></a><br /><a href="https://github.com/acouvreur/sablier/commits?author=romdum" title="Code">💻</a> <a href="#ideas-romdum" title="Ideas, Planning, & Feedback">🤔</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Add your contributions</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->