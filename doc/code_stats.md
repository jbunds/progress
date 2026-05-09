[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Sat May  9 22:37:00 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                        9      1055      146        84      825        128           152.56
(ULOC)                                            626
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
progress_test.go                                  422       39         1      382         45            11.78
progress.go                                       421       70        68      283         58            20.49
examples/fractional/main.go                        57       10         1       46          8            17.39
examples/weight-based/main.go                      45        8         1       36          3             8.33
tracker.go                                         39        5         5       29          3            10.34
fraction.go                                        21        3         1       17          1             5.88
unique.go                                          19        3         1       15          3            20.00
standard.go                                        18        5         1       12          7            58.33
percent.go                                         13        3         5        5          0             0.00
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
Total                                    15      1228      167        91      970        128           152.56
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                       737
DRYness %                                        0.60
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $26,164
Estimated Schedule Effort (organic) 3.44 months
Estimated People Required (organic) 0.67
Processed 39829 bytes, 0.040 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
