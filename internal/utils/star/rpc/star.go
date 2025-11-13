package rpc

import (
	"net/url"
	"strings"
)

type RpcOptions struct {
	parameters url.Values
	include    string
	exclude    string
	method     string
}

type RpcOption func(*RpcOptions)

func WithParameter(key string, value string) RpcOption {
	return func(options *RpcOptions) {
		if options.parameters == nil {
			options.parameters = url.Values{}
		}
		options.parameters.Add(key, value)
	}
}

func WithParameters(parameters url.Values) RpcOption {
	return func(options *RpcOptions) {
		options.parameters = parameters
	}
}

func WithInclude(include string) RpcOption {
	return func(options *RpcOptions) {
		options.include = include
	}
}

func WithExclude(exclude string) RpcOption {
	return func(options *RpcOptions) {
		options.exclude = exclude
	}
}

func WithMethod(method string) RpcOption {
	return func(options *RpcOptions) {
		options.method = method
	}
}

func Post(path string, opts ...RpcOption) string {
	return Rpc(path, append(opts, WithMethod("post"))...)
}

func Get(path string, opts ...RpcOption) string {
	return Rpc(path, append(opts, WithMethod("get"))...)
}

func Rpc(path string, opts ...RpcOption) string {
	options := &RpcOptions{
		method: "post",
	}
	for _, opt := range opts {
		opt(options)
	}

	parsedUrl, err := url.Parse("/rpc/" + path)
	if err != nil {
		panic(err)
	}

	if options.parameters != nil {
		parsedUrl.RawQuery = options.parameters.Encode()
	}

	var rpcOptions []string
	var rpcFiltersOptions []string

	if options.include != "" {
		rpcFiltersOptions = append(rpcFiltersOptions, "include: "+options.include)
	}
	if options.exclude != "" {
		rpcFiltersOptions = append(rpcFiltersOptions, "exclude: "+options.exclude)
	}

	if len(rpcFiltersOptions) > 0 {
		rpcOptions = append(rpcOptions, "filterSignals: {"+strings.Join(rpcFiltersOptions, ", ")+"}")
	}

	return "@post('" + parsedUrl.String() + "', {" + strings.Join(rpcOptions, ", ") + "})"
}
