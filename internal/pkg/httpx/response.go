package httpx

import "github.com/gin-gonic/gin"

const SuccessCode = 1000

type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func Success(c *gin.Context, data any) {
	c.JSON(200, Envelope{
		Code: SuccessCode,
		Msg:  "ok",
		Data: data,
	})
}

func Fail(c *gin.Context, code int, msg string, data any) {
	c.JSON(200, Envelope{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
