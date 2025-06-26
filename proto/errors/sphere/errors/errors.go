package errors

func (x *Error) Error() string {
	if x == nil {
		return ""
	}
	return x.GetReason()
}
