package backup

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// 从文件还原到数据库
func Restore(filePath string) error {
	totalTable := len(models.AllTables)
	count := 0
	// 检查是否正在运行
	if IsRunning() {
		return fmt.Errorf("备份或还原任务正在运行中")
	}
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在")
	}
	SetRunning(true)
	defer SetRunning(false)
	// 停止所有任务
	stopAllTasks()
	defer startAllTasks()
	// 解压到临时目录
	tempDir, err := os.MkdirTemp(filepath.Join(helpers.ConfigDir, "backups"), "backup-restore-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)
	// 解压文件
	if zerr := helpers.ExtractZip(filePath, tempDir); zerr != nil {
		return fmt.Errorf("解压文件失败: %v", zerr)
	}
	// 检查tempDir下是否只有一个文件夹
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("读取临时目录失败: %v", err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		helpers.AppLogger.Infof("备份文件解压后只有一个文件夹: %s,使用该文件夹做为入口目录", entries[0].Name())
		tempDir = filepath.Join(tempDir, entries[0].Name())
	}
	// 开始还原
	SetRunningResult("restore", "开始还原数据库", totalTable, count, "", true)
	for _, table := range models.AllTables {
		if err := restoreFromJsonFile(tempDir, helpers.GetStructName(table), totalTable, &count, table); err != nil {
			helpers.AppLogger.Warnf("恢复表 %s 失败: %v", helpers.GetStructName(table), err)
			continue
		}
	}
	helpers.AppLogger.Infof("完成恢复任务")
	return nil
}

// 从json文件还原到数据库
func restoreFromJsonFile(backupDir string, modelName string, totalTable int, count *int, model any) error {
	backupFilePath := filepath.Join(backupDir, modelName+".json")
	// 检查文件是否存在
	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		helpers.AppLogger.Warnf("备份文件不存在: %s", backupFilePath)
		return nil
	}
	// 读取文件，一行是一条json
	file, err := os.Open(backupFilePath)
	if err != nil {
		helpers.AppLogger.Warnf("打开备份文件 %s 失败: %v", backupFilePath, err)
		return fmt.Errorf("打开备份文件 %s 失败: %v", backupFilePath, err)
	}
	defer file.Close()
	// 1. 删除表（如果存在）
	err = db.Db.Migrator().DropTable(model)
	if err != nil {
		// 处理错误
		helpers.AppLogger.Warnf("删除表 %s 失败: %v", modelName, err)
		return fmt.Errorf("删除表 %s 失败: %v", modelName, err)
	} else {
		helpers.AppLogger.Infof("表 %s 已删除", modelName)
	}

	// 2. 重新创建表
	err = db.Db.AutoMigrate(model)
	if err != nil {
		// 处理错误
		helpers.AppLogger.Warnf("创建表 %s 失败: %v", modelName, err)
		if strings.Contains(err.Error(), "index") {
			helpers.AppLogger.Infof("表 %s 索引创建失败，跳过错误，继续导入数据", modelName)
		} else {
			return fmt.Errorf("创建表 %s 失败: %v", modelName, err)
		}
	} else {
		helpers.AppLogger.Infof("表 %s 已创建", modelName)
	}
	// 读取文件内容
	scanner := bufio.NewScanner(file)
	// 统计还原数量
	var restoredCount int
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	setCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		// 使用反射创建新实例
		item := reflect.New(typ).Interface()
		if err := json.Unmarshal([]byte(line), item); err != nil {
			helpers.AppLogger.Warnf("%s 解析json失败: %v", modelName, err)
		} else {
			// 插入数据库
			if err := db.Db.Create(item).Error; err != nil {
				helpers.AppLogger.Warnf("%s 插入数据库失败: %v", modelName, err)
			}
		}
		restoredCount++
		setCount++
		if setCount >= 10 {
			setCount = 0
			SetRunningResult("restore", fmt.Sprintf("已还原 %d 条 %s 记录", restoredCount, modelName), totalTable, *count, "", false)
		}
	}
	tableName := models.GetTableName(model)
	// 重置表的主键序列
	models.ResetSequence(tableName, "id")
	*count++
	SetRunningResult("restore", fmt.Sprintf("已还原 %d 条 %s 记录", restoredCount, modelName), totalTable, *count, "", false)
	return nil
}
