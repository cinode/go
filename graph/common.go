package graph

func panicOn(condition bool, message string) {
	if condition {
		panic(message)
	}
}
