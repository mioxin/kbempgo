package worker

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/mioxin/kbempgo/internal/utils"
)

type AvatarInfo struct {
	ActualName string
	Num        int
	Size       int64
	Hash       string
}

func NewAvatarInfo(path string) (AvatarInfo, error) {
	var (
		fileInfo os.FileInfo
		err      error
		num      int
	)

	sNum := ""

	fileInfo, err = os.Stat(path)
	if err != nil {
		return AvatarInfo{}, err
	}

	slNum := strings.Split(strings.Split(fileInfo.Name(), ".")[0], " ")
	if len(slNum) > 1 {
		sNum = utils.FindBetween(slNum[1], "(", ")")
		if sNum != "" {
			num, err = strconv.Atoi(sNum)
			if err != nil {
				slog.Error("getFileCollection:", "err", err, "name", fileInfo.Name(), "sNum", sNum)
			}
		}
	}

	hash, err := HashFile(path)
	if err != nil {
		return AvatarInfo{}, err
	}

	return AvatarInfo{
		ActualName: fileInfo.Name(),
		Num:        num,
		Size:       fileInfo.Size(),
		Hash:       hash,
	}, nil
}

// HashFile calculate xxHash of file
func HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed open file %s: %w", filePath, err)
	}
	defer file.Close() // nolint

	hasher := xxhash.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("ошибка чтения файла %s: %w", filePath, err)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	return hash, nil
}
