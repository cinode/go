package blenc

func errPanic(e error) {
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
}
