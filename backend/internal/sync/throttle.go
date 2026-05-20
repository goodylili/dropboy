package sync

import (
	"io"
	"time"
)

// throttledReader caps read throughput to bytesPerSec. It is used to honour
// config.Limits.MaxUploadMbps on the engine's upload path. Zero or negative
// rates disable throttling.
type throttledReader struct {
	r           io.Reader
	bytesPerSec int64
	bucket      float64
	last        time.Time
}

func newThrottledReader(r io.Reader, bytesPerSec int64) io.Reader {
	if bytesPerSec <= 0 {
		return r
	}
	return &throttledReader{r: r, bytesPerSec: bytesPerSec, bucket: float64(bytesPerSec), last: time.Now()}
}

func (t *throttledReader) Read(p []byte) (int, error) {
	// Refill bucket based on time elapsed.
	now := time.Now()
	t.bucket += now.Sub(t.last).Seconds() * float64(t.bytesPerSec)
	if t.bucket > float64(t.bytesPerSec) {
		t.bucket = float64(t.bytesPerSec)
	}
	t.last = now

	// If the bucket is empty, sleep until we accumulate at least one
	// reasonable chunk (16 KiB or the request size, whichever is smaller).
	want := int64(len(p))
	if want > 16<<10 {
		want = 16 << 10
	}
	if t.bucket < float64(want) {
		need := float64(want) - t.bucket
		time.Sleep(time.Duration(need / float64(t.bytesPerSec) * float64(time.Second)))
		now = time.Now()
		t.bucket = float64(want)
		t.last = now
	}

	n, err := t.r.Read(p[:want])
	t.bucket -= float64(n)
	return n, err
}

func mbpsToBytes(mbps int) int64 {
	if mbps <= 0 {
		return 0
	}
	// Megabits per second → bytes per second.
	return int64(mbps) * 1_000_000 / 8
}
