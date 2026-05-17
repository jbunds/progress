[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Mon May 18 00:36:28 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       20      2402      287       171     1944        262           264.46
(ULOC)                                           1390
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  397       40         4      353         51            14.45
render_test.go                                    336       27         0      309         25             8.09
progress.go                                       269       46        62      161         17            10.56
render.go                                         245       38        64      143         28            19.58
tracker_test.go                                   164        9         0      155         19            12.26
term_test.go                                      143       18         0      125         22            17.60
format_test.go                                    134       11         1      122         18            14.75
format.go                                         124       14        10      100         22            22.00
term.go                                            83       12        10       61         17            27.87
themes.go                                          78        7         1       70          7            10.00
examples/fractional/main.go                        76       12         4       60         10            16.67
themes_test.go                                     73        4         0       69          4             5.80
examples/weight-based/main.go                      56        9         1       46          4             8.70
tracker.go                                         47       11         5       31          2             6.45
layout.go                                          44        5         1       38          0             0.00
unique.go                                          36        6         1       29          6            20.69
standard.go                                        33        7         1       25          6            24.00
fraction.go                                        24        3         1       20          1             5.00
percent.go                                         21        4         5       12          0             0.00
init_test.go                                       19        4         0       15          3            20.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      7       235       20         9      206          0             0.00
(ULOC)                                            158
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.golangci.yml                                      61        1         4       56          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      32        4         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.github/workflows/vulncheck.yml                    26        2         0       24          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    27      2637      307       180     2150        262           264.46
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1547
DRYness %                                        0.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $60,347
Estimated Schedule Effort (organic) 4.73 months
Estimated People Required (organic) 1.13
Processed 86607 bytes, 0.087 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
