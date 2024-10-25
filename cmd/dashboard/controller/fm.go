package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-uuid"
	"github.com/naiba/nezha/model"
	"github.com/naiba/nezha/pkg/utils"
	"github.com/naiba/nezha/pkg/websocketx"
	"github.com/naiba/nezha/proto"
	"github.com/naiba/nezha/service/rpc"
	"github.com/naiba/nezha/service/singleton"
)

// Create FM session
// @Summary Create FM session
// @Description Create an "attached" FM. It is advised to only call this within a terminal session.
// @Tags auth required
// @Accept json
// @Param id path uint true "Server ID"
// @Produce json
// @Success 200 {object} model.CreateFMResponse
// @Router /file [get]
func createFM(c *gin.Context) (*model.CreateFMResponse, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return nil, err
	}

	streamId, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	rpc.NezhaHandlerSingleton.CreateStream(streamId)

	singleton.ServerLock.RLock()
	server := singleton.ServerList[id]
	singleton.ServerLock.RUnlock()
	if server == nil || server.TaskStream == nil {
		return nil, errors.New("server not found or not connected")
	}

	fmData, _ := utils.Json.Marshal(&model.TaskFM{
		StreamID: streamId,
	})
	if err := server.TaskStream.Send(&proto.Task{
		Type: model.TaskTypeFM,
		Data: string(fmData),
	}); err != nil {
		return nil, err
	}

	return &model.CreateFMResponse{
		SessionID: streamId,
	}, nil
}

// Start FM stream
// @Summary Start FM stream
// @Description Start FM stream
// @Tags auth required
// @Param id path string true "Stream UUID"
// @Router /ws/file/{id} [get]
func fmStream(c *gin.Context) (any, error) {
	streamId := c.Param("id")
	if _, err := rpc.NezhaHandlerSingleton.GetStream(streamId); err != nil {
		return nil, err
	}
	defer rpc.NezhaHandlerSingleton.CloseStream(streamId)

	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}
	defer wsConn.Close()
	conn := websocketx.NewConn(wsConn)

	go func() {
		// PING 保活
		for {
			if err = conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
			time.Sleep(time.Second * 10)
		}
	}()

	if err = rpc.NezhaHandlerSingleton.UserConnected(streamId, conn); err != nil {
		return nil, err
	}

	return nil, rpc.NezhaHandlerSingleton.StartStream(streamId, time.Second*10)
}