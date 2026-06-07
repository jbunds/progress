package progress

func init() { // override the default / production no-op stubs to facilitate synchronous, deterministic testing

	isTestEnvironment = true

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
