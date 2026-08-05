package image

type PreallocationMode string

const (
	PreallocationDisabled PreallocationMode = "disabled"
	PreallocationSparse   PreallocationMode = "sparse"
)
