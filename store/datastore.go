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
	db        *bolt.DB
	role      string
	certStore *stow.Store
	serverCS  *stow.Store
}

////////////////////////////////////////////////////////////////////////////////
func New(id, role, storePath string) (*DataStore, error) {

	storePath = path.Join(storePath, fmt.Sprintf("%s.store.db", id))
	db, err := bolt.Open(storePath, 0600, nil)
	if err != nil {
		return nil, err
	}

	certStore := stow.NewStore(db, []byte("certs"))
	store := &DataStore{
		db:        db,
		role:      role,
		certStore: certStore,
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
func (d *DataStore) Pool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	err := d.certStore.ForEach(func(id string, entry CertEntry) {
		if ok := pool.AppendCertsFromPEM(entry.Data); !ok {
			logging.Logger.Errorf("unable to add %s certificate for id %q to pool", d.role, id)
		}
	})
	if err != nil {
		return nil, errors.Annotate(err, "enumerate cert entries")
	}

	if len(pool.Subjects()) == 0 {
		return nil, errors.Errorf("no %s certificates stored", d.role)
	}

	return pool, nil
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) StoreCert(id string, certPath string) error {
	data, err := ioutil.ReadFile(certPath)
	if err != nil {
		return errors.Annotatef(err, "load %s cert file", d.role)
	}
	entry := CertEntry{}
	if err := d.certStore.Get(id, &entry); err == nil {
		return errors.Errorf("certificate for %s id %q already stored", d.role, id)
	}

	entry.Data = data
	return d.certStore.Put(id, entry)
}

////////////////////////////////////////////////////////////////////////////////
func (d *DataStore) RemoveCert(id string) error {
	entry := CertEntry{}
	if err := d.certStore.Get(id, &entry); err != nil {
		return errors.Errorf("certificate for %s id %q not available", d.role, id)
	}

	if err := d.certStore.Delete(id); err != nil {
		return errors.Annotatef(err, "delete certificate for %s id %q", d.role, id)
	}

	return nil
}
