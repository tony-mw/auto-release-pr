package utils

type Utility interface {
	Check() bool
}

type Logger struct {
	Debug bool
}

type Error struct {
	Fatal bool
}

func (l Logger) Check() bool {
	if l.Debug {
		return true
	}
	return false
}

func(e Error) Check() bool {
	if e.Fatal {
		return true
	}
	return false
}

func CheckUtil(u Utility) bool {
	return u.Check()
}