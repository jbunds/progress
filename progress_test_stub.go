//go:build test
package progress

type Progress struct{}

func NewProgress(total int) *Progress { return &Progress{} }

func (p *Progress) Update(input string) {}

func (p *Progress) Close() {}
