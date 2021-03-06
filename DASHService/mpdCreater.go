package DASHService

import (
	"encoding/xml"
	"fmt"
	"time"
    "strconv"
	"github.com/use-go/websocket-streamserver/logger"
	"github.com/use-go/websocket-streamserver/mediaTypes/flv"	
	"github.com/use-go/websocket-streamserver/utils"
	"github.com/use-go/websocket-streamserver/mediaTypes/aac"
	"github.com/use-go/websocket-streamserver/mediaTypes/h264"
)

// const string for media description
const (
	ProfileFull        = "urn:mpeg:dash:profile:full:2011"
	ProfileISOOnDemand = "urn:mpeg:dash:profile:isoff-on-demand:2011"
	ProfileISOMain     = "urn:mpeg:dash:profile:isoff-main:2011"
	ProfileISOLive     = "urn:mpeg:dash:profile:isoff-live:2011"
	ProfileTSMain      = "urn:mpeg:dash:profile:mp2t-main:2011"
	ProfileTSSimple    = "urn:mpeg:dash:profile:mp2t-simple:2011"

	staticMPD  = "static"
	dynamicMPD = "dynamic"
	MPDXMLNS   = "urn:mpeg:dash:schema:mpd:2011"

	SchemeIdUri = "urn:mpeg:dash:23003:3:audio_channel_configuration:2011"
)

// MPD Struct
type MPD struct {
	ID                        string      `xml:"id,attr"`
	Profiles                  string      `xml:"profiles,attr"`
	Type                      string      `xml:"type,attr"`
	AvailabilityStartTime     string      `xml:"availabilityStartTime,attr"`
	PublishTime               string      `xml:"publishTime,attr"`
	MediaPresentationDuration string      `xml:"mediaPresentationDuration,attr,omitempty"`
	MinimumUpdatePeriod       string      `xml:"minimumUpdatePeriod,attr,omitempty"`
	MinBufferTime             string      `xml:"minBufferTime,attr"`
	Xmlns                     string      `xml:"xmlns,attr"`
	Period                    []PeriodXML `xml:"Period"`
}

type PeriodXML struct {
	ID            string             `xml:"id,attr"`
	AdaptationSet []AdaptationSetXML `xml:"AdaptationSet"`
}

type AdaptationSetXML struct {
	Lang                      string                        `xml:"lang,attr,omitempty"`
	MimeType                  string                        `xml:"mimeType,attr"`
	Codecs                    string                        `xml:"codecs,attr,omitempty"`
	AudioChannelConfiguration *AudioChannelConfigurationXML `xml:"AudioChannelConfiguration,omitempty"`
	SegmentTemplate           SegmentTemplateXML            `xml:"SegmentTemplate"`
	Representation            []RepresentationXML           `xml:"Representation,omitempty"`
}

type SegmentTemplateXML struct {
	Media           string              `xml:"media,attr"`
	Initialization  string              `xml:"initialization,attr"`
	Duration        *int                `xml:"duration,attr,omitempty"`
	StartNumber     string              `xml:"startNumber,attr"`
	TimeScale       string              `xml:"timescale,attr"`
	SegmentTimeline *SegmentTimelineXML `xml:"SegmentTimeline,omitempty"`
}

//SegmentTimelineXML from file
type SegmentTimelineXML struct {
	S []SegmentTimelineDesc `xml:"S"`
}

// SegmentTimelineDesc from file
type SegmentTimelineDesc struct {
	T string `xml:"t,attr,omitempty"` //time
	D string `xml:"d,attr"`           //duration
	R string `xml:"r,attr,omitempty"` //repreat count default 0
}

//RepresentationXML file
type RepresentationXML struct {
	ID                string `xml:"id,attr"`
	Bandwidth         string `xml:"bandwidth,attr"`
	Width             string `xml:"width,attr,omitempty"`
	Height            string `xml:"height,attr,omitempty"`
	FrameRate         string `xml:"frameRate,attr,omitempty"`
	AudioSamplingRate string `xml:"audioSamplingRate,attr,omitempty"`
}

//AudioChannelConfigurationXML file
type AudioChannelConfigurationXML struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
	Value       int    `xml:"value,attr"`
}

type mpdCreater struct {
	audioHeader  *flv.FlvTag
	videoHeader  *flv.FlvTag
	avaStartTime string
}

func (dashMpdCreater *mpdCreater) init(videoHeader, audioHeader *flv.FlvTag) {
	dashMpdCreater.audioHeader = audioHeader
	dashMpdCreater.videoHeader = videoHeader
	t := time.Now()
	dashMpdCreater.avaStartTime = t.Format("2006-01-02T15:04:05.000Z")
}

func generatePTime(year, month, day, hour, minute, sec, mill int) string {
	str := fmt.Sprintf("P%dY%dM%dDT%dH%dM", year, month, day, hour, minute)
	str += fmt.Sprintf("%.3fS", float32(sec+mill/1000.0))
	return str
}

func (dashMpdCreater *mpdCreater) GetXML(id string, startNumber int) (buf []byte) {
	mpd := &MPD{ID: id,
		Profiles: ProfileISOLive,
		Type:     dynamicMPD,
		AvailabilityStartTime: dashMpdCreater.avaStartTime}
	t := time.Now()
	mpd.PublishTime = t.Format("2006-01-02T15:04:05.000Z")
	//MediaPresentationDuration ignore
	mpd.MinimumUpdatePeriod = generatePTime(0, 0, 0, 0, 0, 3, 0)
	mpd.MinBufferTime = generatePTime(0, 0, 0, 0, 0, 1, 0)
	mpd.Xmlns = MPDXMLNS
	mpd.Period = dashMpdCreater.createPeriod(startNumber)

	buf, err := xml.Marshal(mpd)

	if err != nil {
		logger.LOGE(err.Error())
		return nil
	}

	data := make([]byte, len(buf)+len(xml.Header))
	copy(data, []byte(xml.Header))
	copy(data[len([]byte(xml.Header)):], buf)
	return data
}

func (dashMpdCreater *mpdCreater) createPeriod(startNumber int) (period []PeriodXML) {

	period = make([]PeriodXML, 0, 2)

	if dashMpdCreater.videoHeader != nil {
		videPeroid := PeriodXML{ID: utils.GenerateGUID()}
		dashMpdCreater.createVidePeroid(startNumber, &videPeroid)
		period = append(period, videPeroid)
	}

	if dashMpdCreater.audioHeader != nil {
		audioPeriod := PeriodXML{ID: utils.GenerateGUID()}
		dashMpdCreater.createAudioPeroid(startNumber, &audioPeriod)
		period = append(period, audioPeriod)
	}

	if len(period) == 0 {
		return nil
	}
	return period
}

func (dashMpdCreater *mpdCreater) createVidePeroid(startNumber int, period *PeriodXML) {
	ada := make([]AdaptationSetXML, 1)
	ada[0].MimeType = "video/mp4"
	var width, height, fps int
	if dashMpdCreater.videoHeader.Data[0] == 0x17 && dashMpdCreater.videoHeader.Data[1] == 0 {
		ada[0].Codecs = "avc1."
		sps, _ := h264.GetSpsPpsFromAVC(dashMpdCreater.videoHeader.Data[5:])
		str := fmt.Sprintf("%x", sps[1])
		if len(str) == 1 {
			ada[0].Codecs += "0"
		}
		ada[0].Codecs += str

		str = fmt.Sprintf("%x", sps[2])
		if len(str) == 1 {
			ada[0].Codecs += "0"
		}
		ada[0].Codecs += str

		str = fmt.Sprintf("%x", sps[3])
		if len(str) == 1 {
			ada[0].Codecs += "0"
		}
		ada[0].Codecs += str
		width, height, fps = h264.ParseSPS(sps)
	}

	ada[0].SegmentTemplate.Media = "../video/$RepresentationID$/$Number$.m4s"
	ada[0].SegmentTemplate.Initialization = "../video/$RepresentationID$/init.mp4"
	ada[0].SegmentTemplate.TimeScale = "1000"
	ada[0].SegmentTemplate.StartNumber = strconv.Itoa(startNumber)
	ada[0].SegmentTemplate.SegmentTimeline = dashMpdCreater.createSegmentTimeLine()

	ada[0].Representation = make([]RepresentationXML, 1)
	ada[0].Representation[0].ID = strconv.Itoa(width) + "_" + strconv.Itoa(height)
	ada[0].Representation[0].Bandwidth = strconv.Itoa(width * 1000)
	ada[0].Representation[0].Width = strconv.Itoa(width)
	ada[0].Representation[0].Height = strconv.Itoa(height)
	ada[0].Representation[0].FrameRate = strconv.Itoa(fps)

	period.AdaptationSet = ada
}

func (dashMpdCreater *mpdCreater) createAudioPeroid(startNumber int, period *PeriodXML) {
	ada := make([]AdaptationSetXML, 1)

	ada[0].MimeType = "audio/mp4"
	ada[0].Lang = "en"
	sampleFreq := 0
	channel := 0
	if dashMpdCreater.audioHeader.Data[0]>>4 == flv.SoundFormatAAC {
		asc := aac.MP4AudioGetConfig(dashMpdCreater.audioHeader.Data[2:])
		ada[0].Codecs = "mp4a.40."
		ada[0].Codecs += strconv.Itoa(asc.Object_type)
		sampleFreq = asc.Sample_rate
		channel = asc.Channels
	}

	ada[0].AudioChannelConfiguration = &AudioChannelConfigurationXML{}
	ada[0].AudioChannelConfiguration.SchemeIdUri = SchemeIdUri
	ada[0].AudioChannelConfiguration.Value = channel

	ada[0].SegmentTemplate.Media = "../video/$RepresentationID$/$Number$.m4s"
	ada[0].SegmentTemplate.Initialization = "../video/$RepresentationID$/init.mp4"
	ada[0].SegmentTemplate.TimeScale = strconv.Itoa(sampleFreq)
	ada[0].SegmentTemplate.StartNumber = strconv.Itoa(startNumber)
	ada[0].SegmentTemplate.SegmentTimeline = dashMpdCreater.createSegmentTimeLine()

	ada[0].Representation = make([]RepresentationXML, 1)
	ada[0].Representation[0].ID = "1_stereo"
	ada[0].Representation[0].Bandwidth = "12800"
	ada[0].Representation[0].AudioSamplingRate = strconv.Itoa(sampleFreq)

	period.AdaptationSet = ada
}

func (dashMpdCreater *mpdCreater) createSegmentTimeLine() (segTm *SegmentTimelineXML) {
	//bad time line
	segTm = &SegmentTimelineXML{}
	segTm.S = make([]SegmentTimelineDesc, 1)
	segTm.S[0].D = "1000"

	return
}
