[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue May 26 18:29:03 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       23      2958      365       362     2231        300           295.12
(ULOC)                                           1734
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  399       40         4      355         51            14.37
render_test.go                                    339       27         0      312         25             8.01
progress.go                                       330       57        83      190         22            11.58
render.go                                         264       44        67      153         28            18.30
progress_bench_test.go                            215       24        95       96          7             7.29
integration_test.go                               182       22        54      106         20            18.87
tracker_test.go                                   155        9         0      146         18            12.33
term_test.go                                      145       18         0      127         22            17.32
format_test.go                                    134       11         1      122         18            14.75
format.go                                         124       14        10      100         22            22.00
term.go                                            89       13        13       63         17            26.98
themes.go                                          78        7         1       70          7            10.00
examples/fractional/main.go                        75       13         2       60         10            16.67
themes_test.go                                     73        4         0       69          4             5.80
examples/weight-based/main.go                      62       10         6       46          4             8.70
standard.go                                        59        9         2       48         12            25.00
tracker.go                                         45       10         5       30          2             6.67
pool.go                                            42        7         8       27          2             7.41
fraction.go                                        38        7         3       28          0             0.00
unique.go                                          36        8         2       26          6            23.08
layout.go                                          34        3         1       30          0             0.00
percent.go                                         21        4         5       12          0             0.00
init_test.go                                       19        4         0       15          3            20.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      7       239       20         9      210          0             0.00
(ULOC)                                            162
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.golangci.yml                                      65        1         4       60          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      32        4         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/vulncheck.yml                    26        2         0       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    30      3197      385       371     2441        300           295.12
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1895
DRYness %                                        0.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $68,951
Estimated Schedule Effort (organic) 4.98 months
Estimated People Required (organic) 1.23
Processed 106933 bytes, 0.107 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
