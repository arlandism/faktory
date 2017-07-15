package storage

import (
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBasicQueueOps(t *testing.T) {
	t.Parallel()
	defer os.RemoveAll("../tmp/queues.db")

	store, err := Open("rocksdb", "queues.db")
	assert.NoError(t, err)
	defer store.Close()
	q, err := store.GetQueue("default")
	assert.NoError(t, err)

	assert.Equal(t, int64(0), q.Size())

	data, err := q.Pop()
	assert.NoError(t, err)
	assert.Nil(t, data)

	err = q.Push([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, int64(1), q.Size())

	data, err = q.Pop()
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)
	assert.Equal(t, int64(0), q.Size())
}

func TestDecentQueueUsage(t *testing.T) {
	defer os.RemoveAll("../tmp/qbench.db")
	store, err := Open("rocksdb", "qbench.db")
	assert.NoError(t, err)
	q, err := store.GetQueue("default")
	assert.NoError(t, err)

	assert.Equal(t, int64(0), q.Size())
	err = q.Push([]byte("first"))
	n := 50000
	// Push N jobs to queue
	// Get Size() each time
	for i := 0; i < n; i++ {
		_, data := fakeJob()
		err = q.Push(data)
		assert.NoError(t, err)
		assert.Equal(t, int64(i+2), q.Size())
	}
	err = q.Push([]byte("last"))
	assert.Equal(t, int64(n+2), q.Size())
	// Close DB, reopen
	store.Close()

	store, err = Open("rocksdb", "qbench.db")
	assert.NoError(t, err)
	q, err = store.GetQueue("default")
	assert.NoError(t, err)

	// Pop N jobs from queue
	// Get Size() each time
	assert.Equal(t, int64(n+2), q.Size())
	data, err := q.Pop()
	assert.NoError(t, err)
	assert.Equal(t, []byte("first"), data)
	for i := 0; i < n; i++ {
		_, err := q.Pop()
		assert.NoError(t, err)
		assert.Equal(t, int64(n-i), q.Size())
	}
	data, err = q.Pop()
	assert.NoError(t, err)
	assert.Equal(t, []byte("last"), data)
	assert.Equal(t, int64(0), q.Size())

	data, err = q.Pop()
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestThreadedQueueUsage(t *testing.T) {
	defer os.RemoveAll("../tmp/qthreaded.db")
	store, err := Open("rocksdb", "qthreaded.db")
	assert.NoError(t, err)
	q, err := store.GetQueue("default")
	assert.NoError(t, err)

	tcnt := 10
	n := 5000

	var wg sync.WaitGroup
	for i := 0; i < tcnt; i++ {
		go func() {
			wg.Add(1)
			defer wg.Done()
			pushAndPop(t, n, q)
		}()
	}

	time.Sleep(5 * time.Millisecond)
	wg.Wait()
	assert.Equal(t, int64(0), q.Size())
	q.(*RocksQueue).Init()
	q.Each(func(k, v []byte) error {
		log.Println(string(k), string(v))
		return nil
	})
	store.Close()
}

func pushAndPop(t *testing.T, n int, q Queue) {
	for i := 0; i < n; i++ {
		_, data := fakeJob()
		err := q.Push(data)
		assert.NoError(t, err)
	}

	for i := 0; i < n; i++ {
		_, err := q.Pop()
		assert.NoError(t, err)
	}
}

func TestQueueKeys(t *testing.T) {
	t.Parallel()

	q := &RocksQueue{
		Name: "foo",
		high: 1293712938,
	}
	x := q.nextkey()
	y := q.nextkey()
	z := q.nextkey()
	assert.Equal(t, x[0:3], []byte("foo"))
	assert.Equal(t, x[3], byte(255))
	assert.Equal(t, int64(1293712938), toInt64(x[4:12]))
	assert.Equal(t, int64(1293712939), toInt64(y[4:12]))
	assert.Equal(t, int64(1293712940), toInt64(z[4:12]))

	x = q.nextkey()
	assert.Equal(t, x[0:3], []byte("foo"))
	assert.Equal(t, int64(1293712941), toInt64(x[4:12]))
}
