package xendit

import (
	"context"
	"errors"
	"fmt"
	"time"

	xendit "github.com/xendit/xendit-go/v7"
	"github.com/xendit/xendit-go/v7/payment_request"
)

type QRCodeResult struct {
	ID          string    `json:"id"`
	ReferenceID string    `json:"reference_id,omitempty"`
	QRString    string    `json:"qr_string"`
	Status      string    `json:"status,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Client struct {
	sdk           *xendit.APIClient
	callbackToken string
}

func NewClient(apiKey, callbackToken string) *Client {
	return &Client{
		sdk:           xendit.NewClient(apiKey),
		callbackToken: callbackToken,
	}
}

func (c *Client) CreateQRIS(ctx context.Context, orderRef string, amount int64, description string) (*QRCodeResult, error) {
	qrChannelCode := payment_request.QRCODECHANNELCODE_QRIS
	qrCodeParams := payment_request.NewQRCodeParameters()
	qrCodeParams.SetChannelCode(qrChannelCode)

	pmType := payment_request.PAYMENTMETHODTYPE_QR_CODE
	reusability := payment_request.PAYMENTMETHODREUSABILITY_ONE_TIME_USE

	pmParams := payment_request.NewPaymentMethodParameters(pmType, reusability)
	pmParams.SetQrCode(*qrCodeParams)

	params := *payment_request.NewPaymentRequestParameters(payment_request.PaymentRequestCurrency("IDR"))

	params.SetReferenceId(orderRef)
	params.SetAmount(float64(amount))
	params.SetDescription(description)
	params.SetPaymentMethod(*pmParams)

	resp, _, err := c.sdk.PaymentRequestApi.CreatePaymentRequest(ctx).IdempotencyKey(orderRef).PaymentRequestParameters(params).Execute()
	if err != nil {
		return nil, fmt.Errorf("xendit: failed to create payment request: %w", err)
	}

	pm := resp.GetPaymentMethod()
	qr := pm.GetQrCode()
	cp := qr.GetChannelProperties()

	qrString := cp.GetQrString()
	if qrString == "" {
		return nil, errors.New("xendit: QR code string is empty in response")
	}

	expiresAt := time.Now().Add(30 * time.Minute)
	if exp, ok := cp.GetExpiresAtOk(); ok && exp != nil {
		expiresAt = *exp
	}

	return &QRCodeResult{
		ID:          resp.GetId(),
		QRString:    qrString,
		ReferenceID: resp.GetReferenceId(),
		Status:      string(resp.GetStatus()),
		ExpiresAt:   expiresAt,
	}, nil
}

func (c *Client) GetPaymentRequest(ctx context.Context, paymentRequestID string) (string, error) {
	resp, _, err := c.sdk.PaymentRequestApi.GetPaymentRequestByID(ctx, paymentRequestID).Execute()
	if err != nil {
		return "", fmt.Errorf("xendit: failed to get payment request: %w", err)
	}
	return string(resp.GetStatus()), nil
}

func (c *Client) VerifyCallbackToken(token string) bool {
	return token != "" && token == c.callbackToken
}
