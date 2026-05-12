[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue May 12 11:21:33 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       11      1561      191        84     1286        183           158.84
(ULOC)                                            885
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  671       59         0      612         76            12.42
progress.go                                       414       70        68      276         52            18.84
tracker_test.go                                   159        9         0      150         18            12.00
examples/fractional/main.go                        73       12         1       60         10            16.67
examples/weight-based/main.go                      56        9         1       46          4             8.70
tracker.go                                         47       11         5       31          2             6.45
layout.go                                          32        3         1       28          0             0.00
standard.go                                        32        6         1       25         12            48.00
unique.go                                          32        5         1       26          8            30.77
fraction.go                                        24        3         1       20          1             5.00
percent.go                                         21        4         5       12          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      6       173       21         7      145          0             0.00
(ULOC)                                            112
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      35        7         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.golangci.yml                                      22        1         2       19          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    17      1734      212        91     1431        183           158.84
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                       996
DRYness %                                        0.57
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $39,356
Estimated Schedule Effort (organic) 4.02 months
Estimated People Required (organic) 0.87
Processed 52734 bytes, 0.053 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
