[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue May 19 03:32:54 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       22      2696      333       232     2131        291           294.52
(ULOC)                                           1557
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  397       39         4      354         51            14.41
render_test.go                                    338       27         0      311         25             8.04
progress.go                                       305       52        67      186         21            11.29
render.go                                         268       40        67      161         32            19.88
tracker_test.go                                   165        9         0      156         19            12.18
term_test.go                                      145       18         0      127         22            17.32
format_test.go                                    134       11         1      122         18            14.75
format.go                                         124       14        10      100         22            22.00
progress_bench_test.go                            110       15        38       57          4             7.02
term.go                                            88       13        13       62         17            27.42
themes.go                                          78        7         1       70          7            10.00
integration/integration_test.go                    77       12        12       53         12            22.64
examples/fractional/main.go                        75       13         2       60         10            16.67
themes_test.go                                     73        4         0       69          4             5.80
examples/weight-based/main.go                      58       10         2       46          4             8.70
standard.go                                        51        9         1       41         12            29.27
tracker.go                                         47       11         5       31          2             6.45
layout.go                                          44        5         1       38          0             0.00
fraction.go                                        40        7         2       31          0             0.00
unique.go                                          39        9         1       29          6            20.69
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
Total                                    29      2935      353       241     2341        291           294.52
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1718
DRYness %                                        0.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $65,988
Estimated Schedule Effort (organic) 4.90 months
Estimated People Required (organic) 1.20
Processed 95703 bytes, 0.096 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
