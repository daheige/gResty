package gResty

// go http client support get,post,delete,patch,put,head method
// author:daheige

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	resty "github.com/go-resty/resty/v2"
)

//默认请求超时
var DefaultReqTimeout time.Duration = 5 * time.Second

// Service 请求句柄设置
type Service struct {
	BaseUri         string        //请求地址url的前缀
	Timeout         time.Duration //请求超时限制
	Proxy           string        //请求设置的http_proxy代理
	EnableKeepAlive bool          //开始开启长连接
}

// ReqOpt 请求参数设置
type ReqOpt struct {
	Params  map[string]interface{} //get,delete的Params参数
	Data    map[string]interface{} //post请求的data表单数据
	Headers map[string]interface{} //header头信息

	//cookie参数设置
	Cookies        map[string]interface{} //cookie信息
	CookiePath     string                 //可选参数
	CookieDomain   string                 //cookie domain可选
	CookieMaxAge   int                    //cookie MaxAge
	CookieHttpOnly bool                   //cookie httpOnly

	//支持post,put,patch以json格式传递,[]int{1, 2, 3},map[string]string{"a":"b"}格式
	//json支持[],{}数据格式,主要是golang的基本数据类型，就可以
	Json interface{}
}

// Reply 请求后的结果
type Reply struct {
	Err  error  //请求过程中，发生的error
	Body []byte //返回的body内容
}

// ApiStdRes 标准的api返回格式
type ApiStdRes struct {
	Code    int
	Message string
	Data    interface{}
}

// ParseData 解析ReqOpt Params和Data
func (ReqOpt) ParseData(d map[string]interface{}) map[string]string {
	dLen := len(d)
	if dLen == 0 {
		return nil
	}

	//对d参数进行处理
	data := make(map[string]string, dLen)
	for k, v := range d {
		if val, ok := v.(string); ok {
			data[k] = val
		} else {
			data[k] = fmt.Sprintf("%v", v)
		}
	}

	return data
}

// Do 请求方法
// method string  请求的方法get,post,put,patch,delete,head等
// uri    string  请求的相对地址，如果BaseUri为空，就必须是完整的url地址
// opt 	  *ReqOpt 请求参数ReqOpt
func (s *Service) Do(method string, reqUrl string, opt *ReqOpt) *Reply {
	if opt == nil {
		opt = &ReqOpt{}
	}

	if s.BaseUri != "" {
		reqUrl = strings.TrimRight(s.BaseUri, "/") + "/" + reqUrl
	}

	if s.Timeout == 0 {
		s.Timeout = DefaultReqTimeout
	}

	if reqUrl == "" {
		return &Reply{
			Err: errors.New("request url is empty"),
		}
	}

	//短连接的形式请求api
	//关于如何关闭http connection
	//https://www.cnblogs.com/cobbliu/p/4517598.html

	//创建请求客户端
	client := resty.New()
	client = client.SetTimeout(s.Timeout) //timeout设置

	if !s.EnableKeepAlive {
		client = client.SetHeader("Connection", "close") //显示指定短连接
	}

	if s.Proxy != "" {
		client = client.SetProxy(s.Proxy)
	}

	//设置cookie
	if cLen := len(opt.Cookies); cLen > 0 {
		cookies := make([]*http.Cookie, cLen)
		for k, _ := range opt.Cookies {
			cookies = append(cookies, &http.Cookie{
				Name:     k,
				Value:    fmt.Sprintf("%v", opt.Cookies[k]),
				Path:     opt.CookiePath,
				Domain:   opt.CookieDomain,
				MaxAge:   opt.CookieMaxAge,
				HttpOnly: opt.CookieHttpOnly,
			})
		}

		client = client.SetCookies(cookies)
	}

	//设置header头
	if len(opt.Headers) > 0 {
		client = client.SetHeaders(opt.ParseData(opt.Headers))
	}

	var resp *resty.Response
	var err error

	method = strings.ToLower(method)
	switch method {
	case "get", "delete", "head":
		client = client.SetQueryParams(opt.ParseData(opt.Params))
		if method == "get" {
			resp, err = client.R().Get(reqUrl)
			return s.GetResult(resp, err)
		}

		if method == "delete" {
			resp, err = client.R().Delete(reqUrl)
			return s.GetResult(resp, err)
		}

		if method == "head" {
			resp, err = client.R().Head(reqUrl)
			return s.GetResult(resp, err)
		}

	case "post", "put", "patch":
		req := client.R()
		if len(opt.Data) > 0 {
			req = req.SetBody(opt.Data)
		}

		if opt.Json != nil {
			req = req.SetBody(opt.Json)
		}

		if method == "post" {
			resp, err = req.Post(reqUrl)
			return s.GetResult(resp, err)
		}

		if method == "put" {
			resp, err = req.Put(reqUrl)
			return s.GetResult(resp, err)
		}

		if method == "patch" {
			resp, err = req.Patch(reqUrl)
			return s.GetResult(resp, err)
		}

	default:
	}

	return &Reply{
		Err: errors.New("request method not support"),
	}
}

// GetData 处理请求的结果
func (s *Service) GetResult(resp *resty.Response, err error) *Reply {
	res := &Reply{}
	if err != nil {
		res.Err = err
		return res
	}

	//请求返回的body
	res.Body = resp.Body()
	if !resp.IsSuccess() || resp.StatusCode() != 200 {
		res.Err = errors.New("request error: " + fmt.Sprintf("%v", resp.Error()) + "http StatusCode: " + strconv.Itoa(resp.StatusCode()) + "status: " + resp.Status())
		return res
	}

	return res
}

// Text 返回Reply.Body文本格式
func (r *Reply) Text() string {
	return string(r.Body)
}

// Json 将响应的结果Reply解析到data
// 对返回的Reply.Body做json反序列化处理
func (r *Reply) Json(data interface{}) error {
	if len(r.Body) > 0 {
		err := json.Unmarshal(r.Body, data)
		if err != nil {
			return err
		}
	}

	return nil
}
