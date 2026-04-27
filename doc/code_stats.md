[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue Apr 28 00:27:10 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      6       168       21         7      140          0             0.00
(ULOC)                                            107
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      35        7         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
.golangci.yml                                      17        1         2       14          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                        4       829       96        49      684        102            57.42
(ULOC)                                            515
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  378       23         1      354         39            11.02
progress.go                                       348       55        46      247         52            21.05
examples/fractional/main.go                        58       10         1       47          8            17.02
examples/weight-based/main.go                      45        8         1       36          3             8.33
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    10       997      117        56      824        102            57.42
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                       621
DRYness %                                        0.62
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $22,045
Estimated Schedule Effort (organic) 3.23 months
Estimated People Required (organic) 0.61
Processed 31882 bytes, 0.032 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
