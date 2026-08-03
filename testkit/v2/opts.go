package testkit

type Option func(*options)

type options struct {
	parallel bool
}

func Parallel() Option { return func(o *options) { o.parallel = true } }

func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
