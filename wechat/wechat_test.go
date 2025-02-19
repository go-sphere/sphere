package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

type testConfig struct {
	WxMini *Config `json:"wx_mini"`
	Dash   struct {
		Push struct {
			Platform PushTemplateConfig `json:"platform"`
			Withdraw PushTemplateConfig `json:"withdraw"`
		} `json:"Push"`
	} `json:"dash"`
}

func loadTestConfig() (*testConfig, error) {
	var cfg testConfig
	raw, err := os.ReadFile("../../config.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &cfg)
	if err != nil {
		return nil, err
	}
	if cfg.WxMini == nil {
		return nil, fmt.Errorf("config error: wx_mini is nil")
	}
	return &cfg, nil

}

func TestWechat_GetAccessToken(t *testing.T) {
	cfg, err := loadTestConfig()
	if err != nil {
		t.Skip("load config error", err)
		return

	}
	wx := NewWechat(cfg.WxMini)
	token, err := wx.GetAccessToken(context.Background(), true)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(token)
}

func TestWechat_SendMessageWithTemplate(t *testing.T) {
	cfg, err := loadTestConfig()
	if err != nil {
		t.Skip("load config error", err)
		return

	}
	cfg.WxMini.Env = "develop"
	longText := "二十个汉字测试八九十二十个汉字测试八九十二十个汉字测试八九十二十个汉字测试八九十"
	toUser := "oki-t68m0BX3fYs-26iz7pgozWJA"
	wx := NewWechat(cfg.WxMini)

	//msg1 := []any{
	//	//受理编号 {{character_string1.DATA}} 32位以内数字、字母或符号
	//	fmt.Sprintf("%dP%dU%dF", 1, 100000, 1),
	//	//服务名称 {{thing16.DATA}} 20个以内字符
	//	fmt.Sprintf("%s入驻审核", "小红薯"),
	//	//当前进度 {{phrase2.DATA}} 5个以内汉字
	//	"待审核",
	//	//备注 {{thing5.DATA}} 20个以内字符
	//	TruncateString(longText, 20),
	//}
	//t.Log(msg1)
	//err = wx.SendMessageWithTemplate(&cfg.Dash.Push.Platform, msg1, toUser)
	//if err != nil {
	//	t.Error(err)
	//} else {
	//	t.Log("Send message 1 success")
	//}

	amount := 12345
	msg2 := []any{
		//提现金额 {{amount1.DATA}} 1个币种符号+10位以内纯数字，可带小数，结尾可带“元”
		fmt.Sprintf("¥%d.%02d元", amount/100, amount%100),
		//提现类型 {{thing7.DATA}}  20个以内字符	可汉字、数字、字母或符号组合
		"内容收益",
		//审核结果 {{phrase2.DATA}} 5个以内汉字	5个以内纯汉字，例如：配送中
		"待审核",
		//审核时间 {{time4.DATA}}   24小时制时间格式（支持+年月日），支持填时间段，两个时间点之间用“~”符号连接	例如：15:01，或：2019年10月1日 15:01
		time.Now().Format("2006年01月02日 15:04"),
		//备注 {{thing6.DATA}}     20个以内字符	可汉字、数字、字母或符号组合
		TruncateString(longText, 20),
	}
	t.Log(msg2)
	err = wx.SendMessageWithTemplate(context.Background(), &cfg.Dash.Push.Withdraw, msg2, toUser)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Send message 2 success")
	}
}
