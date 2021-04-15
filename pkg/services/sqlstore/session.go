package sqlstore

import (
	"context"
	"reflect"

	"xorm.io/xorm"
)

type DBSession struct {
	*xorm.Session
	events []interface{}
}

type dbTransactionFunc func(sess *DBSession) error

func (sess *DBSession) publishAfterCommit(msg interface{}) {
	sess.events = append(sess.events, msg)
}

// NewSession returns a new DBSession
func (ss *SQLStore) NewSession() *DBSession {
	return &DBSession{Session: ss.engine.NewSession()}
}

func newSession() *DBSession {
	return &DBSession{Session: x.NewSession()}
}

func startSession(ctx context.Context, engine *xorm.Engine, beginTran bool) (*DBSession, error) {
	value := ctx.Value(ContextSessionKey{})
	var sess *DBSession
	sess, ok := value.(*DBSession)

	if ok {
		return sess, nil
	}

	newSess := &DBSession{Session: engine.NewSession()}
	if beginTran {
		err := newSess.Begin()
		if err != nil {
			return nil, err
		}
	}
	return newSess, nil
}

// WithDbSession calls the callback with a session.
func (ss *SQLStore) WithDbSession(ctx context.Context, callback dbTransactionFunc) error {
	return withDbSession(ctx, ss.engine, callback)
}

func withDbSession(ctx context.Context, engine *xorm.Engine, callback dbTransactionFunc) error {
	sess := &DBSession{Session: engine.NewSession()}
	defer func() {
		if err := sess.Close(); err != nil {
			sqlog.Warn("Failed to close session", "error", err)
		}
	}()

	return callback(sess)
}

func (sess *DBSession) InsertId(bean interface{}) (int64, error) {
	table := sess.DB().Mapper.Obj2Table(getTypeName(bean))

	if err := dialect.PreInsertId(table, sess.Session); err != nil {
		return 0, err
	}
	id, err := sess.Session.InsertOne(bean)
	if err != nil {
		return 0, err
	}
	if err := dialect.PostInsertId(table, sess.Session); err != nil {
		return 0, err
	}

	return id, nil
}

func getTypeName(bean interface{}) (res string) {
	t := reflect.TypeOf(bean)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
