package exec

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/basenana/go-flow/cfg"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/utils"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	HttpOperator     = "http"
	EncoderOperator  = "encoder"
	DecoderOperator  = "decoder"
	MySQLOperator    = "mysql"
	PostgresOperator = "postgres"
)

func NewPipeOperatorBuilderRegister() *Registry {
	r := NewEmptyRegistry()
	_ = r.Register(HttpOperator, newPipeHttpOperator)
	_ = r.Register(EncoderOperator, newPipeEncodeOperator)
	_ = r.Register(DecoderOperator, newPipeDecodeOperator)
	_ = r.Register(MySQLOperator, newPipeMySQLOperator)
	_ = r.Register(PostgresOperator, newPipePostgresOperator)

	return r
}

type PipeExecutor struct {
	flow     *flow.Flow
	registry *Registry
	logger   utils.Logger
}

func (p *PipeExecutor) Setup(ctx context.Context) error {
	return nil
}

func (p *PipeExecutor) DoOperation(ctx context.Context, task flow.Task, operatorSpec flow.Spec) error {
	builder, err := p.registry.FindBuilder(operatorSpec.Type)
	if err != nil {
		return err
	}
	operator, err := builder(task, operatorSpec)
	if err != nil {
		p.logger.Errorf("build operator %s failed: %s", operatorSpec.Type, err)
		return err
	}

	param := &flow.Parameter{
		FlowID:  p.flow.ID,
		Workdir: flowWorkdir(cfg.LocalWorkdirBase, p.flow.ID),
		Result:  &flow.ResultData{},
	}
	err = operator.Do(ctx, param)
	if err != nil {
		p.logger.Errorf("run operator %s failed: %s", operatorSpec.Type, err)
		return err
	}
	return nil
}

func (p *PipeExecutor) Teardown(ctx context.Context) {
	return
}

func NewPipeExecutor(flow *flow.Flow, registry *Registry) flow.Executor {
	return &PipeExecutor{flow: flow, registry: registry, logger: utils.NewLogger("pipe").With(flow.ID)}
}

const (
	httpMaxRespBody int64 = 1 << 22 // 4M
)

type pipeHttpOperator struct {
	task       flow.Task
	httpConfig *flow.Http
}

func (h *pipeHttpOperator) Do(ctx context.Context, param *flow.Parameter) error {
	cli, err := h.buildHttpClient()
	if err != nil {
		return err
	}

	req, err := h.buildHttpRequest()
	if err != nil {
		return err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	buf := &bytes.Buffer{}
	if resp.ContentLength < httpMaxRespBody {
		_, err = io.Copy(buf, resp.Body)
		if err != nil {
			return err
		}
	} else {
		buf.WriteString("context too long")
	}
	setTaskResult(param.Result, h.task.Name, buf.String())

	return nil
}

func (h *pipeHttpOperator) buildHttpClient() (*http.Client, error) {
	var (
		client     = &http.Client{}
		httpConfig = h.httpConfig
	)

	// Disable SSL verification if Insecure is true
	if httpConfig.Insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return client, nil
}

func (h *pipeHttpOperator) buildHttpRequest() (*http.Request, error) {
	httpConfig := h.httpConfig

	// Create URL with query parameters
	requestURL, err := url.Parse(httpConfig.URL)
	if err != nil {
		return nil, err
	}
	queryParams := requestURL.Query()
	for key, value := range httpConfig.Query {
		queryParams.Set(key, value)
	}
	requestURL.RawQuery = queryParams.Encode()

	// Create request body
	var requestBody *bytes.Buffer
	if httpConfig.Body != "" {
		requestBody = bytes.NewBuffer([]byte(httpConfig.Body))
	}

	// Create HTTP request
	request, err := http.NewRequest(httpConfig.Method, requestURL.String(), requestBody)
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range httpConfig.Headers {
		request.Header.Set(key, value)
	}

	// Set content type header
	if httpConfig.ContentType != "" {
		request.Header.Set("Content-Type", httpConfig.ContentType)
	}

	return request, nil
}

func newPipeHttpOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	if operatorSpec.Http == nil {
		return nil, fmt.Errorf("http is nil")
	}
	return &pipeHttpOperator{task: task, httpConfig: operatorSpec.Http}, nil
}

type pipeEncodeOperator struct{}

func (e *pipeEncodeOperator) Do(ctx context.Context, param *flow.Parameter) error {
	//TODO implement me
	panic("implement me")
}

func newPipeEncodeOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &pipeEncodeOperator{}, nil
}

type pipeDecodeOperator struct{}

func (d *pipeDecodeOperator) Do(ctx context.Context, param *flow.Parameter) error {
	//TODO implement me
	panic("implement me")
}

func newPipeDecodeOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &pipeDecodeOperator{}, nil
}

type pipeMySQLOperator struct{}

func (l *pipeMySQLOperator) Do(ctx context.Context, param *flow.Parameter) error {
	//TODO implement me
	panic("implement me")
}

func newPipeMySQLOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &pipeMySQLOperator{}, nil
}

type pipePostgresOperator struct{}

func (l *pipePostgresOperator) Do(ctx context.Context, param *flow.Parameter) error {
	//TODO implement me
	panic("implement me")
}

func newPipePostgresOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &pipePostgresOperator{}, nil
}
