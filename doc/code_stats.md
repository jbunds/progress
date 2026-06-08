[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Mon Jun  8 16:27:57 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       27      3372      414       394     2564        327           374.19
(ULOC)                                           1946
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  399       40         4      355         51            14.37
themes.go                                         364       36         7      321          1             0.31
progress.go                                       334       59        67      208         30            14.42
render_test.go                                    329       27         0      302         25             8.28
integration_test.go                               220       24        63      133         25            18.80
progress_bench_test.go                            216       25        95       96          7             7.29
render.go                                         197       37        33      127         25            19.69
tracker_test.go                                   155        9         0      146         18            12.33
term_test.go                                      145       18         0      127         22            17.32
format_test.go                                    104       10         1       93         15            16.13
term.go                                            91       14        13       64         17            26.56
color.go                                           84       14        18       52         15            28.85
examples/fractional/main.go                        77       13         2       62         10            16.13
format.go                                          73        8         8       57         10            17.54
examples/weight-based/main.go                      60       10         2       48          4             8.33
ansi.go                                            57        2        44       11          0             0.00
themes_test.go                                     57        4         0       53          4             7.55
examples/smoke/main.go                             54        7        10       37          3             8.11
standard.go                                        53        8         1       44         19            43.18
examples/flags.go                                  51        4         4       43          4             9.30
layout.go                                          46        5         1       40          0             0.00
tracker.go                                         45       10         5       30          2             6.67
pool.go                                            41        7         7       27          2             7.41
unique.go                                          40        7         1       32         15            46.88
fraction.go                                        38        7         3       28          0             0.00
percent.go                                         21        4         5       12          0             0.00
init_test.go                                       21        5         0       16          3            18.75
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      8       314       21        10      283          0             0.00
(ULOC)                                            194
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.golangci.yml                                     128        1         4      123          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      32        4         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/vulncheck.yml                    26        2         0       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
.github/dependabot.yml                             12        1         1       10          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    35      3686      435       404     2847        327           374.19
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      2139
DRYness %                                        0.58
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $81,040
Estimated Schedule Effort (organic) 5.29 months
Estimated People Required (organic) 1.36
Processed 119150 bytes, 0.119 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
