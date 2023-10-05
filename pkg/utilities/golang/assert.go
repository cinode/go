package golang

func Assert(b bool, message string) {
	if !b {
		panic("Assertion failed: " + message)
	}
}
