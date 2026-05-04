[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Mon May  4 23:54:02 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                        9       989      131        61      797        111           148.61
(ULOC)                                            583
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  417       33         1      383         40            10.44
progress.go                                       362       61        49      252         46            18.25
examples/fractional/main.go                        58       10         1       47          8            17.02
examples/weight-based/main.go                      45        8         1       36          3             8.33
tracker.go                                         39        5         5       29          3            10.34
fraction.go                                        21        3         1       17          1             5.88
unique.go                                          19        3         1       15          3            20.00
standard.go                                        18        5         1       12          7            58.33
percent.go                                         10        3         1        6          0             0.00
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
Total                                    15      1162      152        68      942        111           148.61
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                       694
DRYness %                                        0.60
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $25,371
Estimated Schedule Effort (organic) 3.40 months
Estimated People Required (organic) 0.66
Processed 37518 bytes, 0.038 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
