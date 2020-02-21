// Package zmail is a simple mail sender.
package zmail

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
	"zgo.at/zlog"
)

var (
	SMTP  = ""   // SMTP server connection string.
	Print = true // Print emails to stdout if SMTP if empty.
)

// Send an email.
func Send(subject string, from mail.Address, to []mail.Address, body string) error {
	msg := format(subject, from, to, body)
	toList := make([]string, len(to))
	for i := range to {
		toList[i] = to[i].Address
	}

	switch SMTP {
	case "stdout":
		if Print {
			l := strings.Repeat("═", 50)
			fmt.Println("╔═══ EMAIL " + l + "\n║ " +
				strings.Replace(strings.TrimSpace(string(msg)), "\r\n", "\r\n║ ", -1) +
				"\n╚══════════" + l + "\n")
		}
		return nil
	case "":
		return sendMail(subject, from, toList, msg)
	default:
		return sendRelay(subject, from, toList, msg)
	}
}

var hostname singleflight.Group

// Send direct.
func sendMail(subject string, from mail.Address, to []string, body []byte) error {
	x, _, _ := hostname.Do("hostname", func() (interface{}, error) {
		return os.Hostname()
	})
	hello := x.(string)

	go func() {
		for _, t := range to {
			domain := t[strings.LastIndex(t, "@")+1:]
			mxs, err := net.LookupMX(domain)
			if err != nil {
				zlog.Field("domain", domain).Errorf("zmail sendMail: %s", err)
				return
			}

			for _, mx := range mxs {
				logerr := func(err error) {
					zlog.Fields(zlog.F{
						"host": mx.Host,
						"from": from,
						"to":   to,
					}).Error(err)
				}

				c, err := smtp.Dial(mx.Host + ":25")
				if err != nil {
					logerr(err)
					if strings.Contains(err.Error(), " blocked ") {
						// 14:52:24 ERROR: 554 5.7.1 Service unavailable; Client host [xxx.xxx.xx.xx] blocked using
						// xbl.spamhaus.org.rbl.local; https://www.spamhaus.org/query/ip/xxx.xxx.xx.xx
						break
					}
					continue // Can't connect: try next MX
				}
				defer c.Close()

				err = c.Hello(hello)
				if err != nil {
					logerr(err)
					// Errors from here on are probably fatal error, so just
					// abort.
					// TODO: could improve by checking the status code, but
					// net/smtp doesn't provide them in a good way. This is fine
					// for now as it's intended as a simple backup solution.
					break
				}

				if ok, _ := c.Extension("STARTTLS"); ok {
					err := c.StartTLS(&tls.Config{ServerName: mx.Host})
					if err != nil {
						logerr(err)
						break
					}
				}

				err = c.Mail(from.Address)
				if err != nil {
					logerr(err)
					break
				}
				for _, addr := range to {
					err = c.Rcpt(addr)
					if err != nil {
						logerr(err)
						break
					}
				}

				w, err := c.Data()
				if err != nil {
					logerr(err)
					break
				}
				_, err = w.Write(body)
				if err != nil {
					logerr(err)
					break
				}

				err = w.Close()
				if err != nil {
					logerr(err)
					break
				}

				err = c.Quit()
				if err != nil {
					logerr(err)
					break
				}

				break
			}
		}
	}()
	return nil
}

// Send via relay.
func sendRelay(subject string, from mail.Address, to []string, body []byte) error {
	srv, err := url.Parse(SMTP)
	if err != nil {
		return err
	}

	user := srv.User.Username()
	pw, _ := srv.User.Password()
	host := srv.Host
	if h, _, err := net.SplitHostPort(srv.Host); err == nil {
		host = h
	}

	go func() {
		var auth smtp.Auth
		if user != "" {
			auth = smtp.PlainAuth("", user, pw, host)
		}

		err := smtp.SendMail(srv.Host, auth, from.Address, to, body)
		if err != nil {
			zlog.Fields(zlog.F{
				"host": srv.Host,
				"from": from,
				"to":   to,
			}).Error(errors.Wrap(err, "smtp.SendMail"))
		}
	}()
	return nil
}

var reNL = regexp.MustCompile(`(\r\n){2,}`)

// format a message.
func format(subject string, from mail.Address, to []mail.Address, body string) []byte {
	var msg strings.Builder
	t := time.Now()

	fmt.Fprintf(&msg, "From: %s\r\n", from.String())

	tos := make([]string, len(to))
	for i := range to {
		tos[i] = to[i].String()
	}
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(tos, ","))

	// Create Message-ID
	domain := from.Address[strings.Index(from.Address, "@")+1:]
	hash := fnv.New64a()
	hash.Write([]byte(body))
	rnd, _ := rand.Int(rand.Reader, big.NewInt(0).SetUint64(18446744073709551615))
	msgid := fmt.Sprintf("zmail-%s-%s@%s", strconv.FormatUint(hash.Sum64(), 36),
		strconv.FormatUint(rnd.Uint64(), 36), domain)

	fmt.Fprintf(&msg, "Date: %s\r\n", t.Format(time.RFC1123Z))
	fmt.Fprintf(&msg, "Content-Type: text/plain;charset=utf-8\r\n")
	fmt.Fprintf(&msg, "Content-Transfer-Encoding: quoted-printable\r\n")
	fmt.Fprintf(&msg, "Message-ID: <%s>\r\n", msgid)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", reNL.ReplaceAllString(subject, "")))
	msg.WriteString("\r\n")

	w := quotedprintable.NewWriter(&msg)
	w.Write([]byte(body))
	w.Close()

	return []byte(msg.String())
}
