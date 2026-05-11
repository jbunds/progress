[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue May 12 01:13:12 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       11      1592      194        84     1314        197           160.88
(ULOC)                                            892
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  694       60         0      634         87            13.72
progress.go                                       427       72        68      287         55            19.16
tracker_test.go                                   154        9         0      145         18            12.41
examples/fractional/main.go                        73       12         1       60         10            16.67
examples/weight-based/main.go                      56        9         1       46          4             8.70
tracker.go                                         47       11         5       31          2             6.45
unique.go                                          32        5         1       26          8            30.77
layout.go                                          32        3         1       28          0             0.00
standard.go                                        32        6         1       25         12            48.00
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
Total                                    17      1765      215        91     1459        197           160.88
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1003
DRYness %                                        0.57
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $40,165
Estimated Schedule Effort (organic) 4.05 months
Estimated People Required (organic) 0.88
Processed 53458 bytes, 0.053 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
