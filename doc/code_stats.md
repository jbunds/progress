[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Sun Jun  7 15:29:55 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       27      3434      422       389     2623        342           373.64
(ULOC)                                           1984
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  399       40         4      355         51            14.37
render_test.go                                    335       27         0      308         25             8.12
progress.go                                       334       59        67      208         30            14.42
themes.go                                         333       36         7      290          1             0.34
integration_test.go                               220       24        63      133         25            18.80
progress_bench_test.go                            217       25        96       96          7             7.29
render.go                                         195       36        33      126         25            19.84
tracker_test.go                                   155        9         0      146         18            12.33
term_test.go                                      145       18         0      127         22            17.32
format_test.go                                    135       11         1      123         18            14.63
format.go                                         129       15        11      103         19            18.45
color.go                                           92       15        14       63         18            28.57
term.go                                            91       14        13       64         17            26.56
examples/fractional/main.go                        77       13         2       62         10            16.13
examples/weight-based/main.go                      60       10         2       48          4             8.33
ansi.go                                            57        2        44       11          0             0.00
themes_test.go                                     55        4         0       51          4             7.84
standard.go                                        53        8         1       44         19            43.18
examples/flags.go                                  51        4         4       43          4             9.30
examples/smoke/main.go                             49        7         5       37          3             8.11
layout.go                                          46        5         1       40          0             0.00
tracker.go                                         45       10         5       30          2             6.67
pool.go                                            41        7         7       27          2             7.41
unique.go                                          40        7         1       32         15            46.88
fraction.go                                        38        7         3       28          0             0.00
percent.go                                         21        4         5       12          0             0.00
init_test.go                                       21        5         0       16          3            18.75
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      8       313       21        10      282          0             0.00
(ULOC)                                            193
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.golangci.yml                                     127        1         4      122          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      32        4         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/vulncheck.yml                    26        2         0       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
.github/dependabot.yml                             12        1         1       10          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    35      3747      443       399     2905        342           373.64
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      2176
DRYness %                                        0.58
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $82,774
Estimated Schedule Effort (organic) 5.34 months
Estimated People Required (organic) 1.38
Processed 128432 bytes, 0.128 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
