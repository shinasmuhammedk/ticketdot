package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ticketdot/internal/money"
)

type EmailService struct {
	smtpHost     string
	smtpPort     string
	smtpUser     string
	smtpPassword string
	fromEmail    string
	isConfigured bool
}

type PassengerManifestItem struct {
	SeatNumber string
	Name       string
	Age        int
	Gender     string
}

type TicketEmailData struct {
	BookingRef    string
	PassengerName string
	OperatorName  string
	Origin        string
	Destination   string
	DepartureTime string
	ArrivalTime   string
	BusNumber     string
	BusType       string
	BoardingPoint string
	DroppingPoint string
	ContactPhone  string
	ContactEmail  string
	Seats         []string
	SeatsDisplay  string
	Passengers    []PassengerManifestItem
	Amount        int64
	AmountDisplay string
	PaymentID     string
	PaymentMethod string
	PaymentStatus string
	ConfirmedAt   string
	Year          int
}

type RefundEmailData struct {
	BookingRef          string
	PassengerName       string
	OperatorName        string
	Origin              string
	Destination         string
	SeatsDisplay        string
	RefundID            string
	RefundAmount        int64
	RefundAmountDisplay string
	RefundStatus        string
	RefundTimeline      string
	CancelledAt         string
	Year                int
}

func NewEmailService() *EmailService {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	user := strings.TrimSpace(os.Getenv("SMTP_USER"))
	pass := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))

	if from == "" {
		from = "no-reply@ticketdot.com"
	}
	if port == "" {
		port = "587"
	}

	isConfigured := host != "" && user != "" && pass != ""

	return &EmailService{
		smtpHost:     host,
		smtpPort:     port,
		smtpUser:     user,
		smtpPassword: pass,
		fromEmail:    from,
		isConfigured: isConfigured,
	}
}

const ticketEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Your TicketDot E-Ticket - {{.BookingRef}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px; color: #1e293b; }
    .card { max-width: 620px; margin: 0 auto; background: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05); }
    .header { background: linear-gradient(135deg, #4338ca 0%, #3b82f6 100%); color: #ffffff; padding: 24px; text-align: left; }
    .brand { font-size: 20px; font-weight: 800; letter-spacing: -0.5px; }
    .header-sub { font-size: 12px; opacity: 0.9; margin-top: 4px; }
    .ref-badge { display: inline-block; background: rgba(255, 255, 255, 0.2); padding: 4px 10px; border-radius: 8px; font-family: monospace; font-size: 13px; font-weight: bold; margin-top: 8px; }
    .content { padding: 24px; }
    .route-banner { display: flex; justify-content: space-between; align-items: center; border-bottom: 2px dashed #e2e8f0; padding-bottom: 16px; margin-bottom: 20px; }
    .city-block { font-size: 18px; font-weight: 800; color: #0f172a; }
    .time-block { font-size: 12px; color: #64748b; margin-top: 2px; }
    .arrow { font-size: 18px; color: #6366f1; font-weight: bold; }
    .section-title { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; color: #94a3b8; margin-bottom: 8px; }
    .info-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; margin-bottom: 20px; }
    .info-item { background: #f8fafc; padding: 12px; border-radius: 10px; border: 1px solid #f1f5f9; }
    .info-label { font-size: 11px; color: #64748b; font-weight: 600; text-transform: uppercase; }
    .info-value { font-size: 13px; color: #0f172a; font-weight: 700; margin-top: 2px; }
    .manifest-table { width: 100%; border-collapse: collapse; margin-bottom: 20px; font-size: 12px; }
    .manifest-table th { background: #f1f5f9; padding: 8px 12px; text-align: left; font-weight: 700; color: #475569; }
    .manifest-table td { padding: 8px 12px; border-bottom: 1px solid #f1f5f9; }
    .seat-badge { background: #e0e7ff; color: #4338ca; padding: 2px 8px; border-radius: 6px; font-weight: 800; }
    .points-box { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 12px; padding: 14px; margin-bottom: 20px; }
    .point-row { display: flex; align-items: flex-start; margin-bottom: 8px; }
    .point-dot { width: 8px; height: 8px; border-radius: 50%; margin-top: 4px; margin-right: 8px; }
    .point-text { font-size: 12px; color: #1e293b; }
    .point-sub { font-size: 11px; color: #64748b; }
    .payment-box { background: #ecfdf5; border: 1px solid #a7f3d0; border-radius: 12px; padding: 14px; display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
    .total-paid { font-size: 20px; font-weight: 900; color: #065f46; }
    .footer { background: #f8fafc; border-top: 1px solid #e2e8f0; padding: 16px; text-align: center; font-size: 11px; color: #64748b; }
  </style>
</head>
<body>
  <div class="card">
    <div class="header">
      <div class="brand">TicketDot</div>
      <div class="header-sub">Official Reservation Confirmation & E-Ticket</div>
      <div class="ref-badge">Booking Ref: {{.BookingRef}}</div>
    </div>
    
    <div class="content">
      <div class="route-banner">
        <div>
          <div class="city-block">{{.Origin}}</div>
          <div class="time-block">Departs: {{.DepartureTime}}</div>
        </div>
        <div class="arrow">➔</div>
        <div style="text-align: right;">
          <div class="city-block">{{.Destination}}</div>
          <div class="time-block">Arrives: {{.ArrivalTime}}</div>
        </div>
      </div>

      <div class="info-grid">
        <div class="info-item">
          <div class="info-label">Bus Operator</div>
          <div class="info-value">{{.OperatorName}}</div>
        </div>
        <div class="info-item">
          <div class="info-label">Bus Type & Reg</div>
          <div class="info-value">{{.BusType}} ({{.BusNumber}})</div>
        </div>
        <div class="info-item">
          <div class="info-label">Primary Passenger</div>
          <div class="info-value">{{.PassengerName}}</div>
        </div>
        <div class="info-item">
          <div class="info-label">Reserved Seats</div>
          <div class="info-value"><span class="seat-badge">{{.SeatsDisplay}}</span></div>
        </div>
      </div>

      {{if .Passengers}}
      <div class="section-title">Passenger Manifest</div>
      <table class="manifest-table">
        <thead>
          <tr>
            <th>Seat</th>
            <th>Passenger Name</th>
            <th>Age</th>
            <th>Gender</th>
          </tr>
        </thead>
        <tbody>
          {{range .Passengers}}
          <tr>
            <td><span class="seat-badge">{{.SeatNumber}}</span></td>
            <td><strong>{{.Name}}</strong></td>
            <td>{{if .Age}}{{.Age}} yrs{{else}}—{{end}}</td>
            <td>{{if .Gender}}{{.Gender}}{{else}}—{{end}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{end}}

      <div class="points-box">
        <div class="point-row">
          <div class="point-dot" style="background: #6366f1;"></div>
          <div>
            <div class="point-text"><strong>Boarding:</strong> {{.BoardingPoint}}</div>
            <div class="point-sub">Please report 15 minutes before scheduled departure time</div>
          </div>
        </div>
        <div class="point-row" style="margin-bottom: 0;">
          <div class="point-dot" style="background: #10b981;"></div>
          <div>
            <div class="point-text"><strong>Dropping:</strong> {{.DroppingPoint}}</div>
            <div class="point-sub">Subject to highway traffic regulations</div>
          </div>
        </div>
      </div>

      <div class="payment-box">
        <div>
          <div style="font-size: 11px; font-weight: bold; color: #047857; text-transform: uppercase;">Payment Verified ({{.PaymentStatus}})</div>
          <div style="font-size: 11px; color: #065f46; font-family: monospace;">Txn: {{.PaymentID}} • {{.PaymentMethod}}</div>
        </div>
        <div class="total-paid">{{.AmountDisplay}}</div>
      </div>

      <div style="font-size: 11px; color: #64748b; line-height: 1.5; background: #f1f5f9; padding: 12px; border-radius: 8px;">
        <strong>Important Guidelines:</strong> Please carry an authentic Government-issued Photo ID (Aadhaar, Passport, Driving License, or Voter ID) during travel. For conductor inspection, you may present this digital e-ticket or show your QR boarding pass on the TicketDot app.
      </div>
    </div>

    <div class="footer">
      TicketDot Verified Reservations © {{.Year}} • Safe Travels!
    </div>
  </div>
</body>
</html>
`

const refundEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Booking Cancelled & Refund Initiated - {{.BookingRef}}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px; color: #1e293b; }
    .card { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden; }
    .header { background: #e11d48; color: #ffffff; padding: 20px; text-align: left; }
    .brand { font-size: 18px; font-weight: bold; }
    .content { padding: 24px; }
    .refund-box { background: #fff1f2; border: 1px solid #fecdd3; border-radius: 12px; padding: 16px; margin: 20px 0; }
    .refund-amount { font-size: 24px; font-weight: 900; color: #9f1239; }
    .footer { background: #f8fafc; border-top: 1px solid #e2e8f0; padding: 16px; text-align: center; font-size: 11px; color: #64748b; }
  </style>
</head>
<body>
  <div class="card">
    <div class="header">
      <div class="brand">TicketDot</div>
      <div style="font-size: 12px; opacity: 0.9; margin-top: 4px;">Booking Cancellation Notice</div>
    </div>
    <div class="content">
      <h3 style="color: #0f172a; margin-top: 0;">Reservation Cancelled (Ref: {{.BookingRef}})</h3>
      <p style="font-size: 13px; color: #475569; line-height: 1.5;">
        Dear {{.PassengerName}}, your booking for journey <strong>{{.Origin}} ➔ {{.Destination}}</strong> (Seats: {{.SeatsDisplay}}) has been cancelled and seats have been released back to public inventory.
      </p>

      <div class="refund-box">
        <div style="font-size: 11px; font-weight: 700; color: #9f1239; text-transform: uppercase;">Refund Initiated</div>
        <div class="refund-amount">{{.RefundAmountDisplay}}</div>
        <div style="font-size: 12px; color: #881337; margin-top: 4px; font-family: monospace;">
          Refund Reference ID: <strong>{{.RefundID}}</strong>
        </div>
        <div style="font-size: 11px; color: #4c0519; margin-top: 6px;">
          {{.RefundTimeline}}
        </div>
      </div>

      <p style="font-size: 12px; color: #64748b;">
        If you have questions regarding this refund, our support desk is available 24/7 at support@ticketdot.com.
      </p>
    </div>
    <div class="footer">
      TicketDot Reservations © {{.Year}}
    </div>
  </div>
</body>
</html>
`

func (s *EmailService) SendTicketEmail(toEmail string, data TicketEmailData) error {
	toEmail = strings.TrimSpace(toEmail)
	if toEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	data.Year = time.Now().Year()
	if len(data.Seats) > 0 && data.SeatsDisplay == "" {
		data.SeatsDisplay = strings.Join(data.Seats, ", ")
	}
	if data.AmountDisplay == "" {
		data.AmountDisplay = money.FormatINR(data.Amount)
	}

	tmpl, err := template.New("ticketEmail").Parse(ticketEmailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse ticket template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render ticket template: %w", err)
	}

	subject := fmt.Sprintf("Your TicketDot Boarding Pass & E-Ticket [%s] - %s to %s", data.BookingRef, data.Origin, data.Destination)

	return s.sendEmail(toEmail, subject, body.String(), data.BookingRef, "ticket")
}

func (s *EmailService) SendRefundEmail(toEmail string, data RefundEmailData) error {
	toEmail = strings.TrimSpace(toEmail)
	if toEmail == "" {
		return fmt.Errorf("recipient email is required")
	}

	data.Year = time.Now().Year()
	if data.RefundAmountDisplay == "" {
		data.RefundAmountDisplay = money.FormatINR(data.RefundAmount)
	}

	tmpl, err := template.New("refundEmail").Parse(refundEmailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse refund template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to render refund template: %w", err)
	}

	subject := fmt.Sprintf("Booking Cancelled & Refund Initiated [%s] - %s", data.BookingRef, data.RefundAmountDisplay)

	return s.sendEmail(toEmail, subject, body.String(), data.BookingRef, "refund")
}

func (s *EmailService) sendEmail(toEmail, subject, htmlContent, bookingRef, emailType string) error {
	// If real SMTP credentials are provided, dispatch via SMTP
	if s.isConfigured {
		auth := smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)
		addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

		headers := make(map[string]string)
		headers["From"] = fmt.Sprintf("TicketDot Reservations <%s>", s.fromEmail)
		headers["To"] = toEmail
		headers["Subject"] = subject
		headers["MIME-Version"] = "1.0"
		headers["Content-Type"] = "text/html; charset=UTF-8"

		var msg bytes.Buffer
		for k, v := range headers {
			msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
		msg.WriteString("\r\n")
		msg.WriteString(htmlContent)

		err := smtp.SendMail(addr, auth, s.fromEmail, []string{toEmail}, msg.Bytes())
		if err != nil {
			log.Printf("[EMAIL] SMTP dispatch failed: %v. Storing in local sandbox log.", err)
		} else {
			log.Printf("[EMAIL] Dispatched %s email for booking %s to %s via SMTP", emailType, bookingRef, toEmail)
			return nil
		}
	}

	// Local Development / Sandbox Mode: Save rendered HTML to disk for immediate developer inspection
	outDir := filepath.Join(".", "data", "sent_emails")
	_ = os.MkdirAll(outDir, 0755)
	filePath := filepath.Join(outDir, fmt.Sprintf("%s_%s.html", emailType, bookingRef))
	_ = os.WriteFile(filePath, []byte(htmlContent), 0644)

	log.Printf("[EMAIL SANDBOX] %s email prepared for %s (Ref: %s). Preview saved: %s",
		strings.ToUpper(emailType), toEmail, bookingRef, filePath)

	return nil
}
