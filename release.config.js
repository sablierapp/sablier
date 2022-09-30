module.exports = {
  "branches":  [
    { "name": "main" },
    { "name": "beta", "channel": "beta", "prerelease": "beta" },
  ],
  "plugins": [
    "@semantic-release/commit-analyzer",
    "@semantic-release/release-notes-generator",
    ["@semantic-release/exec", {
      "publishCmd": "make version=${nextRelease.version} release -j 3"
    }],
    ["@semantic-release/github", {
      "assets": [
        "sablier*"
      ]
    }]
  ]
}