package otp

import "testing"

func TestStoreDailyPerPhoneLimit(t *testing.T) {
	s := NewStore(100, 2)
	phone := "+923001111111"

	if _, err := s.Create(phone, PurposeRegister); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	e := s.data[key(phone, PurposeRegister)]
	e.sentAt = e.sentAt.Add(-2 * resendCooldown)
	s.data[key(phone, PurposeRegister)] = e
	s.mu.Unlock()

	if _, err := s.Create(phone, PurposeRegister); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	e = s.data[key(phone, PurposeRegister)]
	e.sentAt = e.sentAt.Add(-2 * resendCooldown)
	s.data[key(phone, PurposeRegister)] = e
	s.mu.Unlock()

	if _, err := s.Create(phone, PurposeRegister); err == nil {
		t.Fatal("expected per-phone daily limit error")
	}
}

func TestStoreVerify(t *testing.T) {
	s := NewStore(10, 10)
	code, err := s.Create("+923009999999", PurposeRegister)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Verify("+923009999999", PurposeRegister, code) {
		t.Fatal("expected verify success")
	}
	if s.Verify("+923009999999", PurposeRegister, code) {
		t.Fatal("OTP should be single-use")
	}
}

func TestNewSenderRequiresTwilio(t *testing.T) {
	if _, err := NewSender(Config{}); err == nil {
		t.Fatal("expected error when Twilio credentials missing")
	}
	sender, err := NewSender(Config{
		TwilioAccountSID: "ACxxxx",
		TwilioAuthToken:  "token",
		TwilioFromNumber: "+15005550006",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sender.Name() != ProviderSMS {
		t.Fatalf("got %q", sender.Name())
	}
}
