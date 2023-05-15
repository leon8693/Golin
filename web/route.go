package web

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"golin/global"
	"golin/run"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GolinHome GolinIndex 单主机首页
func GolinHome(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	indexhtml := strings.Replace(GolinHomeHtml(), "版本", global.Version, -1)
	c.String(http.StatusOK, indexhtml)
}

// GolinIndex 单主机首页
func GolinIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	indexhtml := strings.Replace(IndexHtml(), "版本", global.Version, -1)
	c.String(http.StatusOK, indexhtml)
}

// GolinIndexFile 多主机首页
func GolinIndexFile(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	indexhtml := strings.Replace(IndexFilehtml(), "版本", global.Version, -1)
	c.String(http.StatusOK, indexhtml)
}

// GolinSubmitFile 先获取上传的文件；判断格式是否为xlsx；转换为临时txt文件；通过share函数执行多主机模式；如果有采集成功的主机并且是Windows系统以及来源本机网卡则打开目录窗口
func GolinSubmitFile(c *gin.Context) {
	filename, err := c.FormFile("uploaded-file")
	if err != nil {
		GolinErrorhtml("error", "上传文件失败了哦！选择文件了吗？", c)
	}
	if filepath.Ext(filename.Filename) != ".xlsx" {
		GolinErrorhtml("error", "文件只允许上传xlsx格式哦！！", c)

	}
	tempfilenamexlsx := fmt.Sprintf("%v.xlsx", time.Now().Unix())
	tempfilenametxt := fmt.Sprintf("%v.txt", time.Now().Unix())
	tempfilenamezip := fmt.Sprintf("%v.zip", time.Now().Unix())
	//退出时删除临时文件
	defer func() {
		os.Remove(tempfilenamexlsx)
		os.Remove(tempfilenametxt)
		os.Remove(tempfilenamezip)
	}()
	//保存上传文件
	err = c.SaveUploadedFile(filename, tempfilenamexlsx)
	if err != nil {
		GolinErrorhtml("error", "上传xlsx文件保存失败！", c)
	}
	if CreateTmpTxt(tempfilenamexlsx, tempfilenametxt) {
		mode := c.PostForm("mode")
		var alliplist []string   //预期成功的主机
		var successlist []string //实际成功的主机

		filedata, _ := os.ReadFile(tempfilenametxt)
		for _, s := range strings.Split(string(filedata), "\n") {
			if strings.Count(s, "~~") != 4 {
				continue
			}
			namesplit := strings.Split(s, "~~")
			apendname := filepath.Join(global.Succpath, mode, fmt.Sprintf("%s_%s.log", namesplit[0], namesplit[1]))
			os.Remove(apendname) //删除同名主机记录
			alliplist = append(alliplist, apendname)
		}
		run.Rangefile(tempfilenametxt, "~~", mode) //运行多主机模式
		//如果文件文件则写入到成功主机列表中
		for _, v := range alliplist {
			if global.PathExists(v) {
				successlist = append(successlist, v)
			}
		}
		if len(successlist) == 0 {
			GolinErrorhtml("error", fmt.Sprintf("%d个主机全部执行失败了哦!", len(alliplist)), c)
			c.Abort()
			return
		}
		// 退出时如果sava=false，则删除文件
		defer func() {
			if !save {
				for _, s := range successlist {
					os.Remove(s)
				}
			}
		}()
		err := CreateZipFromFiles(successlist, tempfilenamezip)
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			GolinErrorhtml("error", "打包成zip包失败了！", c)
			c.Abort()
			return
		}
		if len(successlist) > 0 && len(successlist) != len(alliplist) {
			tempfilenamezip = fmt.Sprintf("成功%d个失败%d个主机.zip", len(successlist), len(alliplist)-len(successlist))
			os.Rename(tempfilenamezip, tempfilenamezip)
		}
		sendFile(tempfilenamezip, c)
	}
}

// GolinSubmit 单次提交任务
func GolinSubmit(c *gin.Context) {
	name, ip, user, passwd, port, mode, down := c.PostForm("name"), c.PostForm("ip"), c.PostForm("user"), c.PostForm("password"), c.PostForm("port"), c.PostForm("run_mode"), c.PostForm("down")
	//fmt.Println(name, ip, user, passwd, port, mode)
	savefilename := fmt.Sprintf("%s_%s.log", name, ip)                //保存的文件夹名：名称_ip.log
	successfile := filepath.Join(global.Succpath, mode, savefilename) //保存的完整路径
	if global.PathExists(successfile) {
		GolinErrorhtml("error", "保存的文件中有重名文件，更换一个吧客官~", c)
	}

	switch mode {
	case "Route": //路由模式是单独的
		for _, cmd := range run.Defroutecmd {
			run.Routessh(successfile, ip, user, passwd, port, cmd)
		}
	default: //其他模式统一函数传参
		run.Onlyonerun(fmt.Sprintf("%s~~~%s~~~%s~~~%s~~~%s", name, ip, user, passwd, port), "~~~", mode)
	}

	//run.Onlyonerun(fmt.Sprintf("%s~~~%s~~~%s~~~%s~~~%s", name, ip, user, passwd, port), "~~~", mode)
	if global.PathExists(successfile) {
		//如果不保存文件，文件返回后删除
		defer func() {
			if !save {
				os.Remove(successfile)
			}
		}()
		//down下载文件,preview预览文件
		if down == "down" {
			c.Header("Content-Description", "File Transfer")
			c.Header("Content-Disposition", "attachment; filename="+fmt.Sprintf(fmt.Sprintf("%s_%s(%s).log", name, ip, mode)))
			c.Header("Content-Type", "application/octet-stream")
		}
		//返回文件
		c.File(successfile)
	} else {
		GolinErrorhtml("error", "失败了哦客官~", c)
	}
}

// GolinMondeFileGet 返回模板文件
func GolinMondeFileGet(c *gin.Context) {
	//如果本地没有模板文件则生成一个
	if !global.PathExists(global.XlsxTemplateName) && !CreateTemplateXlsx() {
		c.Header("Content-Type", "text/html; charset=utf-8")
		errhtml := strings.Replace(ErrorHtml(), "status", "error", -1) //替换状态码
		errhtml = strings.Replace(errhtml, "errbody", "模板文件生成失败!", -1) //替换实际错误描述
		c.String(http.StatusOK, errhtml)
	}
	// 返回模板文件
	sendFile(global.XlsxTemplateName, c)
}

func GolinErrorhtml(status, errbody string, c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	errhtml := strings.Replace(ErrorHtml(), "status", status, -1) //替换状态码
	errhtml = strings.Replace(errhtml, "errbody", errbody, -1)    //替换实际错误描述
	c.String(http.StatusOK, errhtml)
}

// sendFile 发生文件
func sendFile(name string, c *gin.Context) {
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.Header("Content-Type", "application/octet-stream")
	c.File(name)
}

func GolinUpdate(c *gin.Context) {
	release, err := global.CheckForUpdate()
	if err != nil {
		GolinErrorhtml("error", "获取最新版本失败,网络不好吧亲～", c)
		c.Abort()
		return
	}
	if release.TagName == global.Version {
		GolinErrorhtml("success", "非常好！当前是最新版本哦~", c)
		c.Abort()
		return
	}
	GolinErrorhtml("update", fmt.Sprintf("<a href='https://github.com/selinuxG/Golin-cli/releases' target='_blank'>当前版本为:%s,最新版本为:%s,点击此处进行更新！</a>", global.Version, release.TagName), c)

}