package progress

func init() { // override the default / production no-op stubs to facilitate synchronous, deterministic testing

	isTestEnvironment = true

	storeLastFrameHook = func(p *Progress, buf []byte) {
		str := string(buf)
		p.lastFrame.Store(&str)
	}

}
