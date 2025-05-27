# How to Contribute

Network Operator is [Apache 2.0 licensed](LICENSE) and accepts contributions via GitHub
pull requests. This document outlines some of the conventions on development
workflow, commit message formatting, contact points and other resources to make
it easier to get your contribution accepted.

## Coding Style

Please follows the standard formatting recommendations and language idioms set out
in [Effective Go](https://golang.org/doc/effective_go.html) and in the
[Go Code Review Comments wiki](https://github.com/golang/go/wiki/CodeReviewComments).

## Certificate of Origin

In order to get a clear contribution chain of trust we use the [signed-off-by language](https://01.org/community/signed-process)
used by the Linux kernel project.

DCO can be found [here](https://developercertificate.org/)

## Format of the patch

Beside the signed-off-by footer, we expect each patch to comply with the following format:

```
Change summary

More detailed explanation of your changes: Why and how.
Wrap it to 72 characters.

Signed-off-by: <contributor@foo.com>
```

## Format of PR title and description

Pull Request title and description are expected to conform to [conventional commits specification](https://www.conventionalcommits.org)
