[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Wed May 13 17:11:53 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       14      1813      226       117     1470        220           223.70
(ULOC)                                           1058
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  675       60         1      614         76            12.38
progress.go                                       249       41        51      157         15             9.55
render.go                                         208       34        29      145         33            22.76
tracker_test.go                                   159        9         0      150         18            12.00
format.go                                         111       16        10       85         21            24.71
term.go                                            93       13        10       70         20            28.57
examples/fractional/main.go                        73       12         1       60         10            16.67
examples/weight-based/main.go                      56        9         1       46          4             8.70
tracker.go                                         47       11         5       31          2             6.45
standard.go                                        33        6         1       26         12            46.15
layout.go                                          32        3         1       28          0             0.00
unique.go                                          32        5         1       26          8            30.77
fraction.go                                        24        3         1       20          1             5.00
percent.go                                         21        4         5       12          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                      6       174       21         7      146          0             0.00
(ULOC)                                            113
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      35        7         2       26          0             0.00
.pre-commit-config.yaml                            27        1         2       24          0             0.00
.golangci.yml                                      23        1         2       20          0             0.00
.github/workflows/validate-workflows.yml           20        2         0       18          0             0.00
.github/workflows/pages.yml                        19        1         0       18          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    20      1987      247       124     1616        220           223.70
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1170
DRYness %                                        0.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $44,715
Estimated Schedule Effort (organic) 4.22 months
Estimated People Required (organic) 0.94
Processed 61875 bytes, 0.062 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
