package errors

func Errorf(format string, args ...any) error {
	return nil
}

func Is(err, target error) bool {
	return false
}

func New(text string) error {
	return nil
}
