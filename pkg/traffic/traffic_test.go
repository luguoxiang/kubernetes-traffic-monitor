package traffic

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTrafficManager(t *testing.T) {
	tm := TrafficManager{}
	t1 := TrafficInfo{
		SrcPort:              123,
		DstPort:              456,
		TcpRequestTimestamp:  []byte{1, 2, 3},
		requestTimestampNano: 5 * 1e9,
	}
	tm.AddRequest(&t1)
	tm.AddRequest(&t1) //ignore duplicate

	assert.Equal(t, tm.allRequests[123], &t1)
	assert.Nil(t, t1.Next)

	assert.NotNil(t, tm.allPackets[5000])
	assert.Equal(t, tm.allPackets[5000].Traffic, &t1)
	assert.Equal(t, tm.allPackets[5000].Timestamp, int64(5000))
	assert.Nil(t, tm.allPackets[5000].Next)

	t2 := TrafficInfo{
		SrcPort:              123,
		DstPort:              456,
		TcpRequestTimestamp:  []byte{1, 2, 4},
		requestTimestampNano: 5 * 1e9,
	}

	tm.AddRequest(&t2)

	assert.Equal(t, tm.allRequests[123], &t2)
	assert.Equal(t, t2.Next, &t1)
	assert.Nil(t, t1.Next)

	assert.NotNil(t, tm.allPackets[5000])
	assert.Equal(t, tm.allPackets[5000].Traffic, &t1)
	assert.Equal(t, tm.allPackets[5000].Timestamp, int64(5000))
	assert.Equal(t, tm.allPackets[5000].Next.Traffic, &t2)
	assert.Equal(t, tm.allPackets[5000].Next.Timestamp, int64(5000))
	assert.Nil(t, tm.allPackets[5000].Next.Next)

	t3 := TrafficInfo{
		SrcPort:              124,
		DstPort:              456,
		TcpRequestTimestamp:  []byte{1, 2, 4},
		requestTimestampNano: (5000 + TIME_RANGE) * 1e6,
	}

	tm.AddRequest(&t3)

	assert.Equal(t, tm.allRequests[124], &t3)
	assert.Nil(t, t3.Next)

	assert.NotNil(t, tm.allPackets[5000])
	assert.Equal(t, tm.allPackets[5000].Traffic, &t3)
	assert.Equal(t, tm.allPackets[5000].Timestamp, int64(5000+TIME_RANGE))
	assert.Nil(t, tm.allPackets[5000].Next)
	assert.Nil(t, tm.allRequests[123])
}
