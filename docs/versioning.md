# Versioning

Sablier follows the [Semantic Versioning 2.0.0](https://semver.org/) Specification (SemVer).

Given a version number MAJOR.MINOR.PATCH, increment the:

  1.  MAJOR version when you make incompatible API changes
  2.  MINOR version when you add functionality in a backwards compatible manner
  3.  PATCH version when you make backwards compatible bug fixes

Additional labels for pre-release and build metadata are available as extensions to the MAJOR.MINOR.PATCH format.

This process is fully automated using [Semantic Release](https://github.com/semantic-release/semantic-release).

The configuration is [release.config.js](https://github.com/acouvreur/sablier/blob/main/release.config.js).