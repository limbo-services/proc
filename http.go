package proc

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
)

func ServeHTTP(addr string, server *http.Server) Runner {
	return func(ctx context.Context) <-chan error {
		out := make(chan error)
		go func() {
			defer close(out)

			var wg = &sync.WaitGroup{}
			server.Handler = httpWaitGroupHandler(wg, server.Handler)

			l, err := net.Listen("tcp", addr)
			if err != nil {
				out <- err
				return
			}
			defer l.Close()

			go func() {
				<-ctx.Done()
				l.Close()
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				err := server.Serve(tcpKeepAliveListener{l.(*net.TCPListener)})
				if err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						out <- err
					}
				}
			}()

			wg.Wait()
		}()
		return out
	}
}

func ServeHTTPS(addr string, server *http.Server) Runner {
	return func(ctx context.Context) <-chan error {
		out := make(chan error)
		go func() {
			defer close(out)

			{
				l, err := net.Listen("tcp", ":0")
				if err != nil {
					out <- err
					return
				}
				l.Close()
				server.Serve(l)
			}

			config := cloneTLSConfig(server.TLSConfig)
			if !strSliceContains(config.NextProtos, "http/1.1") {
				config.NextProtos = append(config.NextProtos, "http/1.1")
			}

			var wg = &sync.WaitGroup{}
			server.Handler = httpWaitGroupHandler(wg, server.Handler)

			l, err := net.Listen("tcp", addr)
			if err != nil {
				out <- err
				return
			}
			defer l.Close()

			go func() {
				<-ctx.Done()
				l.Close()
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				tlsListener := tls.NewListener(tcpKeepAliveListener{l.(*net.TCPListener)}, config)
				err := server.Serve(tlsListener)
				if err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						out <- err
					}
				}
			}()

			wg.Wait()
		}()
		return out
	}
}

func httpWaitGroupHandler(wg *sync.WaitGroup, h http.Handler) http.HandlerFunc {
	if h == nil {
		h = http.DefaultServeMux
	}

	return func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()

		h.ServeHTTP(w, r)
	}
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

// cloneTLSConfig returns a shallow clone of the exported
// fields of cfg, ignoring the unexported sync.Once, which
// contains a mutex and must not be copied.
//
// The cfg must not be in active use by tls.Server, or else
// there can still be a race with tls.Server updating SessionTicketKey
// and our copying it, and also a race with the server setting
// SessionTicketsDisabled=false on failure to set the random
// ticket key.
//
// If cfg is nil, a new zero tls.Config is returned.
func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}
	return &tls.Config{
		Rand:                     cfg.Rand,
		Time:                     cfg.Time,
		Certificates:             cfg.Certificates,
		NameToCertificate:        cfg.NameToCertificate,
		GetCertificate:           cfg.GetCertificate,
		RootCAs:                  cfg.RootCAs,
		NextProtos:               cfg.NextProtos,
		ServerName:               cfg.ServerName,
		ClientAuth:               cfg.ClientAuth,
		ClientCAs:                cfg.ClientCAs,
		InsecureSkipVerify:       cfg.InsecureSkipVerify,
		CipherSuites:             cfg.CipherSuites,
		PreferServerCipherSuites: cfg.PreferServerCipherSuites,
		SessionTicketsDisabled:   cfg.SessionTicketsDisabled,
		SessionTicketKey:         cfg.SessionTicketKey,
		ClientSessionCache:       cfg.ClientSessionCache,
		MinVersion:               cfg.MinVersion,
		MaxVersion:               cfg.MaxVersion,
		CurvePreferences:         cfg.CurvePreferences,
	}
}

func strSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
