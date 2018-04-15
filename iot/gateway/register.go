package gateway

import (
	"encoding/hex"
	"encoding/json"
	"github.com/harveywangdao/road/crypto/md5"
	"github.com/harveywangdao/road/database"
	"github.com/harveywangdao/road/log/logger"
	"github.com/harveywangdao/road/message"
	"github.com/harveywangdao/road/util"
	"time"
)

const (
	RegisterSuccess byte = 0x78
	RegisterFailure byte = 0xA8
	AlreadyRegister byte = 0x79
)

type Register struct {
}

type RegisterReqData struct {
	PerAesKey  uint16
	ThingID    string
	TBoxSN     string
	IMSI       string
	RollNumber uint16
	ICCID      string
}

func (re *Register) genCallbackNum(regReqMsg *message.Message) string {
	regMsgMap := make(map[string]string)
	err := json.Unmarshal(regReqMsg.ServData, &regMsgMap)
	if err != nil {
		logger.Error(err)
		return ""
	}

	data := md5.GenMd5([]byte(regMsgMap["prethingaes128key"] + regMsgMap["rollnumber"]))
	cbn := util.Substr(hex.EncodeToString(data), 0, 16)

	logger.Debug("data =", data, "hex string(data) =", hex.EncodeToString(data))
	logger.Debug("cbn =", cbn)

	return cbn
}

func (re *Register) checkRegisterData(regReqMsg *message.Message) byte {
	result := RegisterFailure //fail

	thingDB, err := database.GetDB(DBName)
	if err != nil {
		logger.Error(err)
		return result
	}

	regMsgMap := make(map[string]interface{})
	err = json.Unmarshal(regReqMsg.ServData, &regMsgMap)
	if err != nil {
		logger.Error(err)
		return result
	}

	logger.Debug("regMsgMap =", regMsgMap)

	var thingserialno, prethingaes128key, thingid, iccid, imsi, thingaes128key string
	var id, status, bid, eventCreationTime int
	err = thingDB.QueryRow("SELECT * FROM thingbaseinfodata_tbl WHERE thingserialno = ?", regMsgMap["thingserialno"]).Scan(
		&id,
		&thingserialno,
		&prethingaes128key,
		&thingid,
		&iccid,
		&imsi,
		&status,
		&bid,
		&thingaes128key,
		&eventCreationTime)

	if err != nil {
		logger.Error(err)
		return result
	}

	if regMsgMap["prethingaes128key"] != prethingaes128key {
		return result
	}

	if regMsgMap["thingid"] != thingid {
		return result
	}

	if regMsgMap["imsi"] != imsi && regMsgMap["iccid"] != iccid {
		return result
	}

	if status == ThingRegisteredUnLogin || status == ThingRegisteredLogined {
		result = AlreadyRegister //already register
		return result
	} else {
		result = RegisterSuccess //success
	}

	return result
}

func (re *Register) registerThing(regReqMsg *message.Message, bid, eventCreationTime uint32, newAesKey string) error {
	thingDB, err := database.GetDB(DBName)
	if err != nil {
		logger.Error(err)
		return err
	}

	regMsgMap := make(map[string]interface{})
	err = json.Unmarshal(regReqMsg.ServData, &regMsgMap)
	if err != nil {
		logger.Error(err)
		return err
	}

	stmtUpd, err := thingDB.Prepare("UPDATE thingbaseinfodata_tbl SET status=?,bid=?,thingaes128key=?,eventcreationtime=? where thingserialno=?")
	if err != nil {
		logger.Error(err)
		return err
	}
	defer stmtUpd.Close()

	_, err = stmtUpd.Exec(ThingRegisteredUnLogin, int(bid), newAesKey, eventCreationTime, regMsgMap["thingserialno"])
	if err != nil {
		logger.Error(err)
		return err
	}

	return nil
}

func (re *Register) getDispatchData(regReqMsg *message.Message, encryptServData []byte, result byte) ([]byte, error) {
	var dd message.DispatchData
	dd.EventCreationTime = regReqMsg.DisPatch.EventCreationTime
	dd.Aid = 0x1
	dd.Mid = 0x2
	dd.MessageCounter = regReqMsg.DisPatch.MessageCounter + 1
	dd.ServiceDataLength = uint16(len(encryptServData))
	dd.Result = result
	dd.SecurityVersion = message.Encrypt_No
	dd.DispatchCreationTime = uint32(time.Now().Unix())

	dispatchData, err := util.StructToByteSlice(dd)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return dispatchData, nil
}

func (re *Register) getMessageHeaderData(serviceData []byte, bid uint32) ([]byte, error) {
	var mh message.MessageHeader
	mh.FixHeader = message.MessageHeaderID
	mh.ServiceDataCheck = util.DataXOR(serviceData)
	mh.ServiceVersion = 0x0 //not sure
	mh.Bid = bid
	mh.MessageFlag = 0x0

	messageHeaderData, err := util.StructToByteSlice(mh)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return messageHeaderData, nil
}

func (re *Register) RegisterACK(conn message.MessageConn, regReqMsg *message.Message) error {
	msg := message.Message{
		Connection: conn,
	}

	var result byte
	var bid uint32 = 0
	var callbackNum string = ""

	//Check data validity
	result = re.checkRegisterData(regReqMsg)
	if result == RegisterSuccess || result == AlreadyRegister {
		callbackNum = re.genCallbackNum(regReqMsg)
		bid = util.GenRandUint32()

		err := re.registerThing(regReqMsg, bid, regReqMsg.DisPatch.EventCreationTime, callbackNum)
		if err != nil {
			logger.Error(err)
			return err
		}
	}

	//Service data
	regACKMsgMap := make(map[string]interface{})
	if result == RegisterSuccess {
		regACKMsgMap["status"] = 0
	} else {
		regACKMsgMap["status"] = 1
	}

	regACKMsgMap["callbacknumber"] = callbackNum
	regACKMsgMap["bid"] = bid

	serviceData, err := json.Marshal(regACKMsgMap)
	if err != nil {
		logger.Error(err)
		return err
	}

	logger.Debug("serviceData =", serviceData)
	logger.Debug("serviceData =", string(serviceData))

	//Encrypy serviceData
	encryptServData := serviceData

	dispatchData, err := re.getDispatchData(regReqMsg, encryptServData, result)
	if err != nil {
		logger.Error(err)
		return err
	}

	messageHeaderData, err := re.getMessageHeaderData(serviceData, bid)
	if err != nil {
		logger.Error(err)
		return err
	}

	err = msg.SendMessage(messageHeaderData, dispatchData, encryptServData)
	if err != nil {
		logger.Error(err)
		return err
	}

	return nil
}