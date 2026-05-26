package progress

func init() { // override the default / production no-op stubs to enable synchronous, deterministic testing

	syncCompleteHook = func(p *Progress) {
		if p.drawNotify != nil {
			select {
			case p.drawNotify <- struct{}{}:
			default:
			}
		}
	}

	storeLastFrameHook = func(p *Progress, buf []byte) {
		str := string(buf)
		p.lastFrame.Store(&str)
	}

}
