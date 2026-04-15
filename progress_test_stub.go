//go:build test
// This is a no-op implementation of the Progress interface used to ensure that terminal-manipulating code (e.g. hiding the cursor) is not executed during unit tests.
package progress

type Progress struct{}

func NewProgress(total int) *Progress { return &Progress{} }

func (p *Progress) Update(input string) {}

func (p *Progress) Close() {}
