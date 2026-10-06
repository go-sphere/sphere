package captcha_test

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/utils/exp/captcha"
)

// lastCodeSender records the most recent code instead of delivering it.
type lastCodeSender struct {
	code string
}

func (s *lastCodeSender) SendCode(number string, code string) error {
	s.code = code
	return nil
}

func ExampleNewManager() {
	sender := &lastCodeSender{}
	manager := captcha.NewManager(captcha.Config{}, sender)

	const number = "+8613800000000"
	if err := manager.SendCode(number); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("code length:", len(sender.code))

	// The default limit is one send per minute per number.
	err := manager.SendCode(number)
	fmt.Println("resend:", err)
	fmt.Println("rate limited:", errors.Is(err, captcha.ErrMinuteLimitExceeded))

	fmt.Println("wrong code:", manager.Verify(number, "not-it"))
	fmt.Println("right code:", manager.Verify(number, sender.code))
	// Codes are single-use.
	fmt.Println("replay:", manager.Verify(number, sender.code))
	// Output:
	// code length: 6
	// resend: minute limit exceeded
	// rate limited: true
	// wrong code: false
	// right code: true
	// replay: false
}
