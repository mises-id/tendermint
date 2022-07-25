package types

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	tmjson "github.com/tendermint/tendermint/libs/json"
)

// a wrapper to emulate a sum type: jsonrpcid = string | int
// TODO: refactor when Go 2.0 arrives https://github.com/golang/go/issues/19412
type jsonrpcid interface {
	isJSONRPCID()
}

// JSONRPCStringID a wrapper for JSON-RPC string IDs
type JSONRPCStringID string

func (JSONRPCStringID) isJSONRPCID()      {}
func (id JSONRPCStringID) String() string { return string(id) }

// JSONRPCIntID a wrapper for JSON-RPC integer IDs
type JSONRPCIntID int

func (JSONRPCIntID) isJSONRPCID()      {}
func (id JSONRPCIntID) String() string { return fmt.Sprintf("%d", id) }

func idFromInterface(idInterface interface{}) (jsonrpcid, error) {
	switch id := idInterface.(type) {
	case string:
		return JSONRPCStringID(id), nil
	case float64:
		// json.Unmarshal uses float64 for all numbers
		// (https://golang.org/pkg/encoding/json/#Unmarshal),
		// but the JSONRPC2.0 spec says the id SHOULD NOT contain
		// decimals - so we truncate the decimals here.
		return JSONRPCIntID(int(id)), nil
	default:
		typ := reflect.TypeOf(id)
		return nil, fmt.Errorf("json-rpc ID (%v) is of unknown type (%v)", id, typ)
	}
}

//----------------------------------------
// REQUEST

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      jsonrpcid       `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
}

// UnmarshalJSON custom JSON unmarshalling due to jsonrpcid being string or int
func (req *RPCRequest) UnmarshalJSON(data []byte) error {
	unsafeReq := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"` // must be map[string]interface{} or []interface{}
	}{}

	err := json.Unmarshal(data, &unsafeReq)
	if err != nil {
		return err
	}

	if unsafeReq.ID == nil { // notification
		return nil
	}

	req.JSONRPC = unsafeReq.JSONRPC
	req.Method = unsafeReq.Method
	req.Params = unsafeReq.Params
	id, err := idFromInterface(unsafeReq.ID)
	if err != nil {
		return err
	}
	req.ID = id

	return nil
}

func NewRPCRequest(id jsonrpcid, method string, params json.RawMessage) RPCRequest {
	return RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func (req RPCRequest) String() string {
	return fmt.Sprintf("RPCRequest{%s %s/%X}", req.ID, req.Method, req.Params)
}

func MapToRequest(id jsonrpcid, method string, params map[string]interface{}) (RPCRequest, error) {
	var paramsMap = make(map[string]json.RawMessage, len(params))
	for name, value := range params {
		valueJSON, err := tmjson.Marshal(value)
		if err != nil {
			return RPCRequest{}, err
		}
		paramsMap[name] = valueJSON
	}

	payload, err := json.Marshal(paramsMap)
	if err != nil {
		return RPCRequest{}, err
	}

	return NewRPCRequest(id, method, payload), nil
}

func ArrayToRequest(id jsonrpcid, method string, params []interface{}) (RPCRequest, error) {
	var paramsMap = make([]json.RawMessage, len(params))
	for i, value := range params {
		valueJSON, err := tmjson.Marshal(value)
		if err != nil {
			return RPCRequest{}, err
		}
		paramsMap[i] = valueJSON
	}

	payload, err := json.Marshal(paramsMap)
	if err != nil {
		return RPCRequest{}, err
	}

	return NewRPCRequest(id, method, payload), nil
}

//----------------------------------------
// RESPONSE

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (err RPCError) Error() string {
	const baseFormat = "RPC error %v - %s"
	if err.Data != "" {
		return fmt.Sprintf(baseFormat+": %s", err.Code, err.Message, err.Data)
	}
	return fmt.Sprintf(baseFormat, err.Code, err.Message)
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      jsonrpcid       `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// UnmarshalJSON custom JSON unmarshalling due to jsonrpcid being string or int
func (resp *RPCResponse) UnmarshalJSON(data []byte) error {
	unsafeResp := &struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *RPCError       `json:"error,omitempty"`
	}{}
	err := json.Unmarshal(data, &unsafeResp)
	if err != nil {
		return err
	}
	resp.JSONRPC = unsafeResp.JSONRPC
	resp.Error = unsafeResp.Error
	resp.Result = unsafeResp.Result
	if unsafeResp.ID == nil {
		return nil
	}
	id, err := idFromInterface(unsafeResp.ID)
	if err != nil {
		return err
	}
	resp.ID = id
	return nil
}

func NewRPCSuccessResponse(id jsonrpcid, res interface{}) RPCResponse {
	var rawMsg json.RawMessage

	if res != nil {
		var js []byte
		js, err := tmjson.Marshal(res)
		if err != nil {
			return RPCInternalError(id, fmt.Errorf("error marshaling response: %w", err))
		}
		rawMsg = json.RawMessage(js)
	}

	return RPCResponse{JSONRPC: "2.0", ID: id, Result: rawMsg}
}

func NewRPCErrorResponse(id jsonrpcid, code int, msg string, data string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg, Data: data},
	}
}

func (resp RPCResponse) String() string {
	if resp.Error == nil {
		return fmt.Sprintf("RPCResponse{%s %X}", resp.ID, resp.Result)
	}
	return fmt.Sprintf("RPCResponse{%s %v}", resp.ID, resp.Error)
}

// From the JSON-RPC 2.0 spec:
//	If there was an error in detecting the id in the Request object (e.g. Parse
// 	error/Invalid Request), it MUST be Null.
func RPCParseError(err error) RPCResponse {
	return NewRPCErrorResponse(nil, -32700, "Parse error. Invalid JSON", err.Error())
}

// From the JSON-RPC 2.0 spec:
//	If there was an error in detecting the id in the Request object (e.g. Parse
// 	error/Invalid Request), it MUST be Null.
func RPCInvalidRequestError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32600, "Invalid Request", err.Error())
}

func RPCMethodNotFoundError(id jsonrpcid) RPCResponse {
	return NewRPCErrorResponse(id, -32601, "Method not found", "")
}

func RPCInvalidParamsError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32602, "Invalid params", err.Error())
}

func RPCInternalError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32603, "Internal error", err.Error())
}

func RPCServerError(id jsonrpcid, err error) RPCResponse {
	return NewRPCErrorResponse(id, -32000, "Server error", err.Error())
}

//----------------------------------------

// WSRPCConnection represents a websocket connection.
type WSRPCConnection interface {
	// GetRemoteAddr returns a remote address of the connection.
	GetRemoteAddr() string
	// WriteRPCResponse writes the response onto connection (BLOCKING).
	WriteRPCResponse(context.Context, RPCResponse) error
	// TryWriteRPCResponse tries to write the response onto connection (NON-BLOCKING).
	TryWriteRPCResponse(RPCResponse) bool
	// Context returns the connection's context.
	Context() context.Context
}

// Context is the first parameter for all functions. It carries a json-rpc
// request, http request and websocket connection.
//
// - JSONReq is non-nil when JSONRPC is called over websocket or HTTP.
// - WSConn is non-nil when we're connected via a websocket.
// - HTTPReq is non-nil when URI or JSONRPC is called over HTTP.
type Context struct {
	// json-rpc request
	JSONReq *RPCRequest
	// websocket connection
	WSConn WSRPCConnection
	// http request
	HTTPReq *http.Request
}

// RemoteAddr returns the remote address (usually a string "IP:port").
// If neither HTTPReq nor WSConn is set, an empty string is returned.
// HTTP:
//		http.Request#RemoteAddr
// WS:
//		result of GetRemoteAddr
func (ctx *Context) RemoteAddr() string {
	if ctx.HTTPReq != nil {
		return ctx.HTTPReq.RemoteAddr
	} else if ctx.WSConn != nil {
		return ctx.WSConn.GetRemoteAddr()
	}
	return ""
}

// Context returns the request's context.
// The returned context is always non-nil; it defaults to the background context.
// HTTP:
//		The context is canceled when the client's connection closes, the request
//		is canceled (with HTTP/2), or when the ServeHTTP method returns.
// WS:
//		The context is canceled when the client's connections closes.
func (ctx *Context) Context() context.Context {
	if ctx.HTTPReq != nil {
		return ctx.HTTPReq.Context()
	} else if ctx.WSConn != nil {
		return ctx.WSConn.Context()
	}
	return context.Background()
}

//----------------------------------------
// SOCKETS

//
// Determine if its a unix or tcp socket.
// If tcp, must specify the port; `0.0.0.0` will return incorrectly as "unix" since there's no port
// TODO: deprecate
func SocketType(listenAddr string) string {
	socketType := "unix"
	if len(strings.Split(listenAddr, ":")) >= 2 {
		socketType = "tcp"
	}
	return socketType
}

//global setter
var TrustIsrgRootX1 = false

func ForceTrustIsrgRootX1() {
	TrustIsrgRootX1 = true
}
func GlobalClientTlsConfig() *tls.Config {
	if !TrustIsrgRootX1 {
		return nil
	}
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	// Append our cert to the system pool
	certs := "-----BEGIN CERTIFICATE-----\n" +
		"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\n" +
		"-----END CERTIFICATE-----"
	if ok := rootCAs.AppendCertsFromPEM([]byte(certs)); !ok {
		fmt.Println("No certs appended, using system certs only")
	}

	config := &tls.Config{
		RootCAs: rootCAs,
	}
	return config
}
