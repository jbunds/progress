[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Fri May 15 04:25:18 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       19      2275      253       136     1886        263           280.72
(ULOC)                                           1316
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  360       23         0      337         43            12.76
render_test.go                                    323       25         0      298         22             7.38
progress.go                                       263       43        58      162         15             9.26
render.go                                         198       32        31      135         33            24.44
tracker_test.go                                   159        9         0      150         18            12.00
term_test.go                                      143       18         0      125         22            17.60
format_test.go                                    134       11         1      122         18            14.75
format.go                                         127       15        12      100         24            24.00
term.go                                            92       13        10       69         20            28.99
themes.go                                          78        7         1       70          7            10.00
examples/fractional/main.go                        76       12         4       60         10            16.67
themes_test.go                                     73        4         0       69          4             5.80
examples/weight-based/main.go                      60        9         5       46          4             8.70
tracker.go                                         47       11         5       31          2             6.45
standard.go                                        33        6         1       26         12            46.15
layout.go                                          32        3         1       28          0             0.00
unique.go                                          32        5         1       26          8            30.77
fraction.go                                        24        3         1       20          1             5.00
percent.go                                         21        4         5       12          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      7       234       20         7      207          0             0.00
(ULOC)                                            157
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.golangci.yml                                      60        1         2       57          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      32        4         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/vulncheck.yml                    26        2         0       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    26      2509      273       143     2093        263           280.72
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1472
DRYness %                                        0.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $58,668
Estimated Schedule Effort (organic) 4.68 months
Estimated People Required (organic) 1.11
Processed 80156 bytes, 0.080 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
