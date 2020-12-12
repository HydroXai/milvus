package storage

import (
	"fmt"
	"os"
	"syscall"

	"github.com/golang/protobuf/proto"
	"github.com/zilliztech/milvus-distributed/internal/errors"
	"github.com/zilliztech/milvus-distributed/internal/proto/internalpb"
	"github.com/zilliztech/milvus-distributed/internal/proto/schemapb"
	"github.com/zilliztech/milvus-distributed/internal/util/tsoutil"
)

func PrintBinlogFiles(fileList []string) error {
	for _, file := range fileList {
		if err := printBinlogFile(file); err != nil {
			return err
		}
	}
	return nil
}

func printBinlogFile(filename string) error {
	fd, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return err
	}
	defer fd.Close()

	fileInfo, err := fd.Stat()
	if err != nil {
		return err
	}

	fmt.Printf("file size = %d\n", fileInfo.Size())

	b, err := syscall.Mmap(int(fd.Fd()), 0, int(fileInfo.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil
	}
	defer syscall.Munmap(b)

	fmt.Printf("buf size = %d\n", len(b))

	r, err := NewBinlogReader(b)
	if err != nil {
		return err
	}
	defer r.Close()

	fmt.Println("descriptor event header:")
	physical, _ := tsoutil.ParseTS(r.descriptorEvent.descriptorEventHeader.Timestamp)
	fmt.Printf("\tTimestamp: %v\n", physical)
	fmt.Printf("\tTypeCode: %s\n", r.descriptorEvent.descriptorEventHeader.TypeCode.String())
	fmt.Printf("\tServerID: %d\n", r.descriptorEvent.descriptorEventHeader.ServerID)
	fmt.Printf("\tEventLength: %d\n", r.descriptorEvent.descriptorEventHeader.EventLength)
	fmt.Printf("\tNextPosition :%d\n", r.descriptorEvent.descriptorEventHeader.NextPosition)
	fmt.Println("descriptor event data:")
	fmt.Printf("\tBinlogVersion: %d\n", r.descriptorEvent.descriptorEventData.BinlogVersion)
	fmt.Printf("\tServerVersion: %d\n", r.descriptorEvent.descriptorEventData.ServerVersion)
	fmt.Printf("\tCommitID: %d\n", r.descriptorEvent.descriptorEventData.CommitID)
	fmt.Printf("\tHeaderLength: %d\n", r.descriptorEvent.descriptorEventData.HeaderLength)
	fmt.Printf("\tCollectionID: %d\n", r.descriptorEvent.descriptorEventData.CollectionID)
	fmt.Printf("\tPartitionID: %d\n", r.descriptorEvent.descriptorEventData.PartitionID)
	fmt.Printf("\tSegmentID: %d\n", r.descriptorEvent.descriptorEventData.SegmentID)
	fmt.Printf("\tFieldID: %d\n", r.descriptorEvent.descriptorEventData.FieldID)
	physical, _ = tsoutil.ParseTS(r.descriptorEvent.descriptorEventData.StartTimestamp)
	fmt.Printf("\tStartTimestamp: %v\n", physical)
	physical, _ = tsoutil.ParseTS(r.descriptorEvent.descriptorEventData.EndTimestamp)
	fmt.Printf("\tEndTimestamp: %v\n", physical)
	dataTypeName, ok := schemapb.DataType_name[int32(r.descriptorEvent.descriptorEventData.PayloadDataType)]
	if !ok {
		return errors.Errorf("undefine data type %d", r.descriptorEvent.descriptorEventData.PayloadDataType)
	}
	fmt.Printf("\tPayloadDataType: %v\n", dataTypeName)
	fmt.Printf("\tPostHeaderLengths: %v\n", r.descriptorEvent.descriptorEventData.PostHeaderLengths)
	eventNum := 0
	for {
		event, err := r.NextEventReader()
		if err != nil {
			return err
		}
		if event == nil {
			break
		}
		fmt.Printf("event %d header:\n", eventNum)
		physical, _ = tsoutil.ParseTS(event.eventHeader.Timestamp)
		fmt.Printf("\tTimestamp: %v\n", physical)
		fmt.Printf("\tTypeCode: %s\n", event.eventHeader.TypeCode.String())
		fmt.Printf("\tServerID: %d\n", event.eventHeader.ServerID)
		fmt.Printf("\tEventLength: %d\n", event.eventHeader.EventLength)
		fmt.Printf("\tNextPosition: %d\n", event.eventHeader.NextPosition)
		switch event.eventHeader.TypeCode {
		case InsertEventType:
			evd, ok := event.eventData.(*insertEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d insert event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printPayloadValues(r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		case DeleteEventType:
			evd, ok := event.eventData.(*deleteEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d delete event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printPayloadValues(r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		case CreateCollectionEventType:
			evd, ok := event.eventData.(*createCollectionEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d create collection event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printDDLPayloadValues(event.eventHeader.TypeCode, r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		case DropCollectionEventType:
			evd, ok := event.eventData.(*dropCollectionEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d drop collection event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printDDLPayloadValues(event.eventHeader.TypeCode, r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		case CreatePartitionEventType:
			evd, ok := event.eventData.(*createPartitionEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d create partition event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printDDLPayloadValues(event.eventHeader.TypeCode, r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		case DropPartitionEventType:
			evd, ok := event.eventData.(*dropPartitionEventData)
			if !ok {
				return errors.Errorf("incorrect event data type")
			}
			fmt.Printf("event %d drop partition event:\n", eventNum)
			physical, _ = tsoutil.ParseTS(evd.StartTimestamp)
			fmt.Printf("\tStartTimestamp: %v\n", physical)
			physical, _ = tsoutil.ParseTS(evd.EndTimestamp)
			fmt.Printf("\tEndTimestamp: %v\n", physical)
			if err := printDDLPayloadValues(event.eventHeader.TypeCode, r.descriptorEvent.descriptorEventData.PayloadDataType, event.PayloadReaderInterface); err != nil {
				return err
			}
		default:
			return errors.Errorf("undefined event typd %d\n", event.eventHeader.TypeCode)
		}
		eventNum++
	}

	return nil
}

func printPayloadValues(colType schemapb.DataType, reader PayloadReaderInterface) error {
	fmt.Println("\tpayload values:")
	switch colType {
	case schemapb.DataType_BOOL:
		val, err := reader.GetBoolFromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %v\n", i, v)
		}
	case schemapb.DataType_INT8:
		val, err := reader.GetInt8FromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %d\n", i, v)
		}
	case schemapb.DataType_INT16:
		val, err := reader.GetInt16FromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %d\n", i, v)
		}
	case schemapb.DataType_INT32:
		val, err := reader.GetInt32FromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %d\n", i, v)
		}
	case schemapb.DataType_INT64:
		val, err := reader.GetInt64FromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %d\n", i, v)
		}
	case schemapb.DataType_FLOAT:
		val, err := reader.GetFloatFromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %f\n", i, v)
		}
	case schemapb.DataType_DOUBLE:
		val, err := reader.GetDoubleFromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			fmt.Printf("\t\t%d : %f\n", i, v)
		}
	case schemapb.DataType_STRING:
		rows, err := reader.GetPayloadLengthFromReader()
		if err != nil {
			return err
		}
		for i := 0; i < rows; i++ {
			val, err := reader.GetOneStringFromPayload(i)
			if err != nil {
				return err
			}
			fmt.Printf("\t\t%d : %s\n", i, val)
		}
	case schemapb.DataType_VECTOR_BINARY:
		val, dim, err := reader.GetBinaryVectorFromPayload()
		if err != nil {
			return err
		}
		dim = dim / 8
		length := len(val) / dim
		for i := 0; i < length; i++ {
			fmt.Printf("\t\t%d :", i)
			for j := 0; j < dim; j++ {
				idx := i*dim + j
				fmt.Printf(" %02x", val[idx])
			}
			fmt.Println()
		}
	case schemapb.DataType_VECTOR_FLOAT:
		val, dim, err := reader.GetFloatVectorFromPayload()
		if err != nil {
			return err
		}
		length := len(val) / dim
		for i := 0; i < length; i++ {
			fmt.Printf("\t\t%d :", i)
			for j := 0; j < dim; j++ {
				idx := i*dim + j
				fmt.Printf(" %f", val[idx])
			}
			fmt.Println()
		}
	default:
		return errors.Errorf("undefined data type")
	}
	return nil
}

func printDDLPayloadValues(eventType EventTypeCode, colType schemapb.DataType, reader PayloadReaderInterface) error {
	fmt.Println("\tpayload values:")
	switch colType {
	case schemapb.DataType_INT64:
		val, err := reader.GetInt64FromPayload()
		if err != nil {
			return err
		}
		for i, v := range val {
			physical, logical := tsoutil.ParseTS(uint64(v))
			fmt.Printf("\t\t%d : physical : %v ; logical : %d\n", i, physical, logical)
		}
	case schemapb.DataType_STRING:
		rows, err := reader.GetPayloadLengthFromReader()
		if err != nil {
			return err
		}
		for i := 0; i < rows; i++ {
			val, err := reader.GetOneStringFromPayload(i)
			if err != nil {
				return err
			}
			switch eventType {
			case CreateCollectionEventType:
				var req internalpb.CreateCollectionRequest
				if err := proto.Unmarshal(([]byte)(val), &req); err != nil {
					return err
				}
				msg := proto.MarshalTextString(&req)
				fmt.Printf("\t\t%d : create collection: %s\n", i, msg)
			case DropCollectionEventType:
				var req internalpb.DropPartitionRequest
				if err := proto.Unmarshal(([]byte)(val), &req); err != nil {
					return err
				}
				msg := proto.MarshalTextString(&req)
				fmt.Printf("\t\t%d : drop collection: %s\n", i, msg)
			case CreatePartitionEventType:
				var req internalpb.CreatePartitionRequest
				if err := proto.Unmarshal(([]byte)(val), &req); err != nil {
					return err
				}
				msg := proto.MarshalTextString(&req)
				fmt.Printf("\t\t%d : create partition: %s\n", i, msg)
			case DropPartitionEventType:
				var req internalpb.DropPartitionRequest
				if err := proto.Unmarshal(([]byte)(val), &req); err != nil {
					return err
				}
				msg := proto.MarshalTextString(&req)
				fmt.Printf("\t\t%d : drop partition: %s\n", i, msg)
			default:
				return errors.Errorf("undefined ddl event type %d", eventType)
			}
		}
	default:
		return errors.Errorf("undefined data type")
	}
	return nil
}
