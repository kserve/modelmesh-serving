// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mmesh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	etcd3kv "go.etcd.io/etcd/api/v3/mvccpb"
	etcd3rpc "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	etcd3 "go.etcd.io/etcd/client/v3"
	etcd3mirror "go.etcd.io/etcd/client/v3/mirror"
	"google.golang.org/grpc"
)

type EtcdConfig struct {
	Endpoints             string `json:"endpoints"`
	Username              string `json:"userid,omitempty"`
	Password              string `json:"password,omitempty"`
	RootPrefix            string `json:"root_prefix,omitempty"`
	Certificate           string `json:"certificate,omitempty"`
	CertificateFile       string `json:"certificate_file,omitempty"`
	ClientKey             string `json:"client_key,omitempty"`
	ClientKeyFile         string `json:"client_key_file,omitempty"`
	ClientCertificate     string `json:"client_certificate,omitempty"`
	ClientCertificateFile string `json:"client_certificate_file,omitempty"`
	OverrideAuthority     string `json:"override_authority,omitempty"`
}

// EtcdRangeWatcher A wrapper for Etcd Watch with common refresh Watch Channel logic
type EtcdRangeWatcher struct {
	logger      logr.Logger
	etcdClient  *etcd3.Client
	WatchPrefix string

	//cache map[string]cacheEntry
}

type KeyEventType uint8

const (
	UPDATE KeyEventType = iota
	DELETE
	INITIALIZED
)

const etcdDialTimeout = 10 * time.Second

var eventTypeNames = []string{
	"UPDATE", "DELETE", "INITIALIZED",
}

func (t KeyEventType) String() string {
	return eventTypeNames[t]
}

type KeyEvent struct {
	Key   string
	Value []byte
	Type  KeyEventType
}

type KeyEventChan chan KeyEvent

type KvListener func(eventType KeyEventType, key string, value []byte)

type cacheEntry struct {
	kv    *etcd3kv.KeyValue
	found bool // This is false during watch phase
}

func NewEtcdRangeWatcher(logger logr.Logger, etcd *etcd3.Client, prefix string) *EtcdRangeWatcher {
	return &EtcdRangeWatcher{
		etcdClient:  etcd,
		logger:      logger.WithName("EtcdRangeWatcher"),
		WatchPrefix: prefix,
		//cache: make(map[string]cacheEntry),
	}
}

func (r *EtcdRangeWatcher) Start(ctx context.Context, keysOnly bool, listener KvListener) {
	log := r.logger.WithValues("WatchPrefix", r.WatchPrefix)
	prefixBytes := len([]byte(r.WatchPrefix))
	log.Info("EtcdRangeWatcher starting")
	go func() {
		initSent := false
		cache := make(map[string]*cacheEntry)
	refresh_loop:
		for {
			client := r.etcdClient
			if keysOnly {
				rokv := &keysOnlyKvAndWatcher{KV: client.KV, Watcher: client.Watcher}
				koClient := *client
				koClient.KV, koClient.Watcher = rokv, rokv
				client = &koClient
			}
			syncer := etcd3mirror.NewSyncer(client, r.WatchPrefix, 0)
			getChan, errChan := syncer.SyncBase(ctx)
			for {
				select {
				case err, ok := <-errChan:
					if !ok {
						errChan = nil
					} else {
						if ctx.Err() != nil {
							break // our context was cancelled
						}
						log.Error(err, "Error refreshing key range, retrying after 3sec")
						//TODO handle this better and identify fatal case
						time.Sleep(3 * time.Second)
						continue refresh_loop
					}
				//TODO determine action based on err
				case gr, ok := <-getChan:
					if !ok {
						getChan = nil
					} else {
						for _, kv := range gr.Kvs {
							k := key(kv, prefixBytes)
							if current, ok := cache[k]; !ok || kv.ModRevision > current.kv.ModRevision {
								cache[k] = &cacheEntry{kv: kv, found: true}
								listener(UPDATE, k, kv.Value)
							} else {
								current.found = true
							}
						}
					}
				}
				if errChan == nil && getChan == nil {
					break
				}
			}
			if !initSent {
				listener(INITIALIZED, "", nil)
				initSent = true
			} else {
				for k, entry := range cache {
					if !entry.found {
						delete(cache, k)
						listener(DELETE, k, entry.kv.Value)
					} else {
						entry.found = false
					}
				}
			}
			// Watch phase
			for wr := range syncer.SyncUpdates(ctx) {
				for _, event := range wr.Events {
					kv := event.Kv
					k := key(kv, prefixBytes)
					switch event.Type {
					case etcd3kv.PUT:
						cache[k] = &cacheEntry{kv: kv}
						listener(UPDATE, k, kv.Value)
					case etcd3kv.DELETE:
						if prev, ok := cache[k]; ok {
							delete(cache, k)
							listener(DELETE, k, prev.kv.Value)
						} else {
							listener(DELETE, k, nil) // unexpected
						}
					}
				}
				err := wr.Err()
				if err != nil {
					if err == etcd3rpc.ErrCompacted {
						// need to resync
						log.Info("Received compacted error")
						break
					}
					log.Error(err, "Watch failure")
					//TODO handle
				}
			}
			if ctx.Err() != nil {
				break // our context was cancelled
			}
		}
	}()
}

func key(kv *etcd3kv.KeyValue, prefixBytes int) string {
	key := kv.Key
	return string(key[prefixBytes:])
}

type keysOnlyKvAndWatcher struct {
	etcd3.KV
	etcd3.Watcher
}

func (k keysOnlyKvAndWatcher) Get(ctx context.Context, key string, opts ...etcd3.OpOption) (*etcd3.GetResponse, error) {
	return k.KV.Get(ctx, key, append(opts, etcd3.WithKeysOnly())...)
}
func (k keysOnlyKvAndWatcher) Watch(ctx context.Context, key string, opts ...etcd3.OpOption) etcd3.WatchChan {
	return k.Watcher.Watch(ctx, key, append(opts, etcd3.WithKeysOnly())...)
}

func CreateEtcdClient(etcdConfig EtcdConfig, secretData map[string][]byte, logger logr.Logger) (*etcd3.Client, error) {
	etcdClientConfig, err := getEtcdClientConfig(etcdConfig, secretData, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client config: %w", err)
	}

	return etcd3.New(*etcdClientConfig)
}

func getEtcdClientConfig(etcdConfig EtcdConfig, secretData map[string][]byte, logger logr.Logger) (*etcd3.Config, error) {
	etcdEndpoints := strings.Split(etcdConfig.Endpoints, ",")

	var tlsConfig tls.Config
	var certificate, clientKey, clientCert []byte
	var useTLS, ok bool

	if strings.HasPrefix(etcdEndpoints[0], "https://") {
		useTLS = true
	}

	if etcdConfig.Certificate != "" || etcdConfig.CertificateFile != "" {
		certString := etcdConfig.Certificate
		certificate = []byte(certString)

		if etcdConfig.CertificateFile != "" {
			if len(certificate) > 0 {
				logger.Info("Ignoring JSON-embedded certificate in favor of dedicated secret key", "key", etcdConfig.CertificateFile)
			}
			if certificate, ok = secretData[etcdConfig.CertificateFile]; !ok {
				return nil, fmt.Errorf("referenced TLS certificate secret key not found: %s", etcdConfig.CertificateFile)
			}
		}

		caCert := x509.NewCertPool()
		caCert.AppendCertsFromPEM(certificate)
		tlsConfig.RootCAs = caCert
		useTLS = true
	}

	if etcdConfig.ClientKey != "" || etcdConfig.ClientKeyFile != "" {
		clientKeyString := etcdConfig.ClientKey
		clientKey = []byte(clientKeyString)

		if etcdConfig.ClientKeyFile != "" {
			if len(clientKey) > 0 {
				logger.Info("Ignoring JSON-embedded client key in favor of dedicated secret key", "key", etcdConfig.ClientKeyFile)
			}
			if clientKey, ok = secretData[etcdConfig.ClientKeyFile]; !ok {
				return nil, fmt.Errorf("referenced TLS key secret key not found: %s", etcdConfig.ClientKeyFile)
			}
		}
	}
	if etcdConfig.ClientCertificate != "" || etcdConfig.ClientCertificateFile != "" {
		clientCertString := etcdConfig.ClientCertificate
		clientCert = []byte(clientCertString)

		if etcdConfig.ClientCertificateFile != "" {
			if len(clientCert) > 0 {
				logger.Info("Ignoring JSON-embedded client cert in favor of dedicated secret key", "key", etcdConfig.ClientCertificateFile)
			}
			if clientCert, ok = secretData[etcdConfig.ClientCertificateFile]; !ok {
				return nil, fmt.Errorf("referenced TLS client certificate secret key not found: %s", etcdConfig.ClientCertificateFile)
			}
		}
	}

	if (len(clientKey) > 0) != (len(clientCert) > 0) {
		return nil, fmt.Errorf("need to set both client_key/client_key_file and client_certificate/client_certificate_file")
	} else if len(clientKey) > 0 {
		tlsCert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("could not load client key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
		useTLS = true
	}

	etcdClientConfig := etcd3.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: etcdDialTimeout,
		Username:    etcdConfig.Username,
		Password:    etcdConfig.Password,
	}

	if useTLS {
		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs, _ = x509.SystemCertPool()
		}
		etcdClientConfig.TLS = &tlsConfig
	}

	if etcdConfig.OverrideAuthority != "" {
		etcdClientConfig.DialOptions = []grpc.DialOption{grpc.WithAuthority(etcdConfig.OverrideAuthority)}
	}

	return &etcdClientConfig, nil
}
