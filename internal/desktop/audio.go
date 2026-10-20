package desktop

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
)

//go:embed sounds/notification.mp3
var builtinSounds embed.FS

const sampleRate = 44100

var builtinIDs = []string{"bell", "chime", "notification", "alarm", "beep"}

type Audio struct {
	mu      sync.RWMutex
	context *oto.Context
	sounds  map[string][]byte
	dir     string
}

func NewAudio(dir string) (*Audio, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	a := &Audio{sounds: make(map[string][]byte), dir: dir}
	for _, id := range []string{"bell", "chime", "alarm", "beep"} {
		a.sounds[id] = synth(id)
	}
	file, err := builtinSounds.ReadFile("sounds/notification.mp3")
	if err != nil {
		return nil, err
	}
	a.sounds["notification"], err = decodeMP3(file)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".mp3" && ext != ".wav" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		pcm, err := decode(data, ext)
		if err == nil {
			a.sounds["custom:"+entry.Name()] = pcm
		}
	}
	return a, nil
}

func (a *Audio) IDs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ids := append([]string(nil), builtinIDs...)
	for id := range a.sounds {
		if strings.HasPrefix(id, "custom:") {
			ids = append(ids, id)
		}
	}
	return ids
}

func (a *Audio) Has(id string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.sounds[id]
	return ok
}

func (a *Audio) Import(name, extension, encoded string) (string, error) {
	extension = strings.ToLower(extension)
	if extension != ".mp3" && extension != ".wav" {
		return "", errors.New("sound must be mp3 or wav")
	}
	if len(encoded) > 3*1024*1024 {
		return "", errors.New("sound must be smaller than 2 MB")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) > 2*1024*1024 {
		return "", errors.New("invalid sound file")
	}
	pcm, err := decode(data, extension)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	name = strings.Map(func(char rune) rune {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			return char
		}
		return '-'
	}, name)
	if name == "" {
		return "", errors.New("sound name is empty")
	}
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), name, extension)
	if err := os.WriteFile(filepath.Join(a.dir, filename), data, 0600); err != nil {
		return "", err
	}
	id := "custom:" + filename
	a.mu.Lock()
	a.sounds[id] = pcm
	a.mu.Unlock()
	return id, nil
}

func (a *Audio) Play(id string) error {
	a.mu.RLock()
	pcm, exists := a.sounds[id]
	a.mu.RUnlock()
	if !exists {
		return errors.New("unknown sound")
	}
	if err := a.Warm(); err != nil {
		return err
	}
	a.mu.Lock()
	player := a.context.NewPlayer(bytes.NewReader(pcm))
	a.mu.Unlock()
	player.Play()
	go func() {
		for player.IsPlaying() {
			time.Sleep(40 * time.Millisecond)
		}
		_ = player.Close()
	}()
	return nil
}

func (a *Audio) Warm() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.context == nil {
		context, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate: sampleRate, ChannelCount: 2, Format: oto.FormatSignedInt16LE,
			BufferSize: 40 * time.Millisecond, ApplicationName: "cryptotape",
		})
		if err != nil {
			return err
		}
		<-ready
		if err := context.Err(); err != nil {
			return err
		}
		a.context = context
	}
	return nil
}

func decode(data []byte, extension string) ([]byte, error) {
	if extension == ".mp3" {
		return decodeMP3(data)
	}
	decoder := wav.NewDecoder(bytes.NewReader(data))
	if !decoder.IsValidFile() {
		return nil, errors.New("invalid wav file")
	}
	buffer, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	if buffer.Format.NumChannels < 1 || buffer.Format.NumChannels > 2 || buffer.Format.SampleRate < 8000 || buffer.Format.SampleRate > 192000 {
		return nil, errors.New("unsupported wav format")
	}
	frames := len(buffer.Data) / buffer.Format.NumChannels
	samples := make([]int16, frames*2)
	shift := buffer.SourceBitDepth - 16
	for i := 0; i < frames; i++ {
		for channel := 0; channel < 2; channel++ {
			input := i*buffer.Format.NumChannels + min(channel, buffer.Format.NumChannels-1)
			value := buffer.Data[input]
			if shift > 0 {
				value >>= shift
			} else if shift < 0 {
				value <<= -shift
			}
			samples[i*2+channel] = int16(max(-32768, min(32767, value)))
		}
	}
	return resample(samples, buffer.Format.SampleRate), nil
}

func decodeMP3(data []byte) ([]byte, error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	pcm, err := io.ReadAll(io.LimitReader(decoder, 40<<20))
	if err != nil {
		return nil, err
	}
	if len(pcm) >= 40<<20 {
		return nil, errors.New("sound is too long")
	}
	if decoder.SampleRate() == sampleRate {
		return pcm, nil
	}
	samples := make([]int16, len(pcm)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
	}
	return resample(samples, decoder.SampleRate()), nil
}

func resample(samples []int16, sourceRate int) []byte {
	if sourceRate <= 0 || len(samples) < 2 {
		return nil
	}
	frames := len(samples) / 2
	outFrames := int(float64(frames) * sampleRate / float64(sourceRate))
	output := make([]byte, outFrames*4)
	for frame := 0; frame < outFrames; frame++ {
		position := float64(frame) * float64(sourceRate) / sampleRate
		left := min(int(position), frames-1)
		right := min(left+1, frames-1)
		fraction := position - float64(left)
		for channel := 0; channel < 2; channel++ {
			value := float64(samples[left*2+channel])*(1-fraction) + float64(samples[right*2+channel])*fraction
			binary.LittleEndian.PutUint16(output[frame*4+channel*2:], uint16(int16(value)))
		}
	}
	return output
}

func synth(id string) []byte {
	duration := 800 * time.Millisecond
	if id == "beep" {
		duration = 150 * time.Millisecond
	}
	if id == "alarm" {
		duration = 400 * time.Millisecond
	}
	frames := int(float64(sampleRate) * duration.Seconds())
	output := make([]byte, frames*4)
	for frame := 0; frame < frames; frame++ {
		t := float64(frame) / sampleRate
		value := 0.0
		switch id {
		case "bell":
			value = .15 * (math.Sin(2*math.Pi*800*t) + math.Sin(2*math.Pi*1200*t)) * math.Exp(-4*t)
		case "chime":
			if t < .3 {
				value += .2 * math.Sin(2*math.Pi*659.25*t) * math.Exp(-8*t)
			}
			if t >= .15 && t < .55 {
				value += .2 * math.Sin(2*math.Pi*523.25*(t-.15)) * math.Exp(-6*(t-.15))
			}
		case "alarm":
			for _, start := range []float64{0, .15, .3} {
				if t >= start && t < start+.1 && math.Sin(2*math.Pi*880*t) >= 0 {
					value += .2
				} else if t >= start && t < start+.1 {
					value -= .2
				}
			}
		case "beep":
			value = .3 * math.Sin(2*math.Pi*600*t) * math.Exp(-20*t)
		}
		value = math.Max(-1, math.Min(1, value))
		sample := uint16(int16(value * 32767))
		binary.LittleEndian.PutUint16(output[frame*4:], sample)
		binary.LittleEndian.PutUint16(output[frame*4+2:], sample)
	}
	return output
}
