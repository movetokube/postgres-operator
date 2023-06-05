package utils

func ListContains(l []string, i string) bool {
	for _, item := range l {
		if item == i {
			return true
		}
	}
	return false
}
