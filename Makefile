auto-hook-pre-push:

test:
	gometalinter -D errcheck -D gas --deadline 20s .
