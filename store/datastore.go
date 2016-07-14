package store

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/juju/errors"

	"github.com/boltdb/bolt"
	"github.com/denkhaus/llconf/logging"
	"github.com/djherbis/stow"
)

type CertEntry struct {
	Data []byte
}

////////////////////////////////////////////////////////////////////////////////
type DataStore struct {
	db       *bolt.DB
	clientCS *stow.Store
	serverCS *stow.Store
}

////////////////////////////////////////////////////////////////////////////////
var (
	ErrNotFound = fmt.Errorf("Datastore:: value not found")
)

////////////////////////////////////////////////////////////////////////////////
func New(storePath string) (*DataStore, error) {
	db, err := bolt.Open(path.Join(storePath, "store.db"), 0600, nil)
	if err != nil {
		return nil, err
	}

	clientCertStore := stow.NewStore(db, []byte("client-certs"))
	serverCertStore := stow.NewStore(db, []byte("server-certs"))

	store := &DataStore{
		db:       db,
		clientCS: clientCertStore,
		serverCS: serverCertStore,
	}

	return store, nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) Close() error {
	if d.db != nil {
		err := d.db.Close()
		d.db = nil
		return err
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) ClientCertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	err := d.clientCS.ForEach(func(clientID string, entry CertEntry) {
		if ok := pool.AppendCertsFromPEM(entry.Data); !ok {
			logging.Logger.Errorf("unable to add client certificate for id %q to pool", clientID)
		}
	})
	if err != nil {
		return nil, errors.Annotate(err, "enumerate cert entries")
	}

	if len(pool.Subjects()) == 0 {
		return nil, errors.New("no client certificates stored")
	}

	return pool, nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) ServerCertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	err := d.serverCS.ForEach(func(serverID string, entry CertEntry) {
		if ok := pool.AppendCertsFromPEM(entry.Data); !ok {
			logging.Logger.Errorf("unable to add server certificate for id %q to pool", serverID)
		}
	})
	if err != nil {
		return nil, errors.Annotate(err, "enumerate cert entries")
	}

	if len(pool.Subjects()) == 0 {
		return nil, errors.New("no server certificates stored")
	}

	return pool, nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) StoreClientCert(clientID string, certPath string) error {
	data, err := ioutil.ReadFile(certPath)
	if err != nil {
		return errors.Annotate(err, "load client cert file")
	}
	entry := CertEntry{}
	if err := d.clientCS.Get(clientID, &entry); err == nil {
		return errors.Errorf("certificate for client id %q already stored", clientID)
	}

	entry.Data = data
	return d.clientCS.Put(clientID, entry)
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) StoreServerCert(serverID string, certPath string) error {
	data, err := ioutil.ReadFile(certPath)
	if err != nil {
		return errors.Annotate(err, "load server cert file")
	}
	entry := CertEntry{}
	if err := d.serverCS.Get(serverID, &entry); err == nil {
		return errors.Errorf("certificate for server id %q already stored", serverID)
	}

	entry.Data = data
	return d.serverCS.Put(serverID, entry)
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) RemoveClientCert(clientID string) error {

	entry := CertEntry{}
	if err := d.clientCS.Get(clientID, &entry); err != nil {
		return errors.Errorf("certificate for client id %q not available", clientID)
	}

	if err := d.clientCS.Delete(clientID); err != nil {
		return errors.Annotatef(err, "delete certificate for client id %q", clientID)
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) RemoveServerCert(serverID string) error {

	entry := CertEntry{}
	if err := d.serverCS.Get(serverID, &entry); err != nil {
		return errors.Errorf("certificate for server id %q not available", serverID)
	}

	if err := d.serverCS.Delete(serverID); err != nil {
		return errors.Annotatef(err, "delete certificate for server id %q", serverID)
	}

	return nil
}
