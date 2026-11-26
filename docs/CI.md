# CI and pre-commit

## GitHub Actions

```yaml
- uses: actions/checkout@<sha>
- uses: Clause-Eight/ato@<sha>
  with:
    path: .
    sarif: ato.sarif      # optional; upload it with github/codeql-action/upload-sarif
```

Findings appear as annotations on the pull request; the step fails on findings unless `fail-on-findings: "false"`. Add `license: ${{ secrets.ATO_LICENSE }}` with your license in a repository or organisation secret.

## pre-commit

```yaml
repos:
  - repo: https://github.com/Clause-Eight/ato
    rev: <tag or sha>
    hooks:
      - id: ato
```

## GitLab and Bitbucket

[ci/gitlab/ato.gitlab-ci.yml](../ci/gitlab/ato.gitlab-ci.yml) (Code Quality widget) and [ci/bitbucket/bitbucket-pipelines.yml](../ci/bitbucket/bitbucket-pipelines.yml). Set your license as a masked, protected CI variable named `ATO_LICENSE`.
