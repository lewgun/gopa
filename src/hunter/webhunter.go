/** 
 * User: Medcl
 * Date: 13-7-8
 * Time: 下午5:42 
 */
package hunter

import (
	"net/http"
	 log "github.com/cihub/seelog"
	"io/ioutil"
	"regexp"
	"util/bloom"
	"sync"
	"util/stringutil"
	"time"
	"strings"
	"os"
.	"net/url"
)

type SiteConfig struct{

	//walking around pattern
	LinkUrlExtractRegex *regexp.Regexp
	LinkUrlMustContain string
	LinkUrlMustNotContain string

	//parsing url pattern,when url match this pattern,gopa will not parse urls from response of this url
	SkipPageParsePattern *regexp.Regexp

	//downloading pattern
	DownloadUrlPattern *regexp.Regexp
	DownloadUrlMustContain string
	DownloadUrlMustNotContain string

	//Crawling within domain
	FollowSameDomain  bool
	FollowSubDomain  bool
}

type  Task struct{
  Url,Request,Response []byte
}

func fetchUrl(url []byte,success chan Task,failure chan string,timeout time.Duration){
	t := time.NewTimer(timeout)
	defer t.Stop()

	resource := string(url)
	flg := make(chan bool, 1)

	go func() {

		defer func () {
			failure <- resource
		}()

		resp, err := http.Get(resource)
		if err != nil {
			log.Error("we have an error!: ", err)
			return
		}
		defer resp.Body.Close()
		log.Debug("getting,", resource)
		body, _ := ioutil.ReadAll(resp.Body)
		task := Task{url,nil, body}

		savePage(url,body)

		success <- task
		flg <- true
	}()

	//监听通道，由于设有超时，不可能泄露
	select {
	case <-t.C:
		log.Error("fetching url time out,",resource)
	case <-flg:
		log.Debug("fetching url normal exit,",resource)
		return
	}

}

func getRootUrl(source *URL)string{
	if(strings.HasSuffix(source.Path,"/")){
		return source.Host+source.Path
	}else{
		index:= strings.LastIndex(source.Path,"/")
		if index>0{
			path:= source.Path[0:index]
			return source.Host+path
		}else{
			return source.Host+"/"
		}
	}
	return ""
}


func savePage(myurl []byte,body []byte){
	myurl1,_:=ParseRequestURI(string(myurl))
	log.Debug("url->path:",myurl1.Host," ",myurl1.Path)

	baseDir:="data/"+myurl1.Host+"/"
	path:=""

	//making folders
	if(strings.HasSuffix(myurl1.Path,"/")){
		path=baseDir+myurl1.Path
		os.MkdirAll(path,0777)
		log.Debug("making dir:",path)
		path=(path+"default.html")
		log.Debug("no page name,use default.html:",path)

	}else{
	    index:= strings.LastIndex(myurl1.Path,"/")
		log.Info("index of last /:",index)
		if index>0{
			path= myurl1.Path[0:index]
			path=baseDir+path
			log.Debug("new path:",path)
			os.MkdirAll(path,0777)
			log.Debug("making dir:",path)
			path=(baseDir+myurl1.Path)
		}else{
			path= baseDir+path+"/"
			os.MkdirAll(path,0777)
			log.Debug("making dir:",path)
			path=path+"/default.html"
		}
	}


	log.Debug("touch file,",path)
	fout,error:=os.Create(path)
	if error!=nil{
		log.Error(path,error)
		return
	}

	defer  fout.Close()
	log.Info("file saved:",path)
	fout.Write(body)

}


func ThrottledCrawl(curl chan []byte,maxGoR int, success chan Task, failure chan string) {
	maxGos := maxGoR
	numGos := 0
	for {
		if numGos > maxGos {
			<-failure
			numGos -= 1
		}
		url := string(<-curl)
//		if _, ok := visited[url]; !ok {
		timeout := 20 * time.Second
		go fetchUrl([]byte(url), success, failure,timeout)
			numGos += 1
//		}
//		visited[url] += 1
	}
}

func Seed(curl chan []byte,seed string) {
	curl <- []byte(seed)
}

var f *bloom.Filter64
var l sync.Mutex

func init(){

//	log.Debug("[webhunter] initializing")

	// Create a bloom filter which will contain an expected 100,000 items, and which
	// allows a false positive rate of 1%.
	f = bloom.New64(1000000, 0.01)

}

func formatUrlForFilter(url []byte) []byte{
	src:=string(url)
	if(strings.HasSuffix(src,"/")){
		src= strings.TrimRight(src,"/");
	}
	src=strings.TrimSpace(src)
	src=strings.ToLower(src)
	return []byte(src)
}

func GetUrls(curl chan []byte, task Task, siteConfig SiteConfig) {

   if(siteConfig.SkipPageParsePattern==nil){
	   siteConfig.SkipPageParsePattern=regexp.MustCompile(".*?\\.((js)|(css)|(rar)|(gz)|(zip)|(exe)|(apk))\\b")   //end with js,css,apk,zip,ignore
	   log.Debug("use default SkipPageParsePattern,",siteConfig.SkipPageParsePattern)
   }

	if(siteConfig.SkipPageParsePattern.Match(task.Url)){
		log.Info("hit skip pattern,",string(task.Url))
		return
	}


	log.Debug("parsing external links:",string(task.Url),",using:",siteConfig.LinkUrlExtractRegex)
	if(siteConfig.LinkUrlExtractRegex==nil){
		siteConfig.LinkUrlExtractRegex=regexp.MustCompile("src=\"(?<url1>.*?)\"|href=\"(?<url2>.*?)\"")
		log.Debug("use default linkUrlExtractRegex,",siteConfig.LinkUrlExtractRegex)
	}

	matches := siteConfig.LinkUrlExtractRegex.FindAllSubmatch(task.Response, -1)
	for _, match := range matches {
		url := match[2]

		log.Debug("original filter url,",string(url))
		filterUrl:=formatUrlForFilter(url)
		log.Debug("format filter url,",string(filterUrl))

		filteredUrl:=string(filterUrl)

		//filter error link
		if(filteredUrl==""){
		  continue;
		}

		result1:=strings.HasPrefix(filteredUrl,"#")
		if(result1){
			continue;
		}

		result2:=strings.HasPrefix(filteredUrl,"javascript:")
		if(result2){
			continue;
		}


		hit := false
//		l.Lock();
//		defer l.Unlock();

		if(f.Test(filterUrl)){
			hit=true
		}

		if(!hit){
			currentUrlStr:=string(url)
			seedUrlStr:=string(task.Url)

			seedURI,err:=ParseRequestURI(seedUrlStr)
			if err != nil {
				log.Error("we have an error!: ", err)
				continue
			}
			currentURI,err:=ParseRequestURI(currentUrlStr)
			if err != nil {
				log.Error("we have an error!: ", err)
				continue
			}


			//relative links
			if(currentURI.Host==""){
			 if(strings.HasPrefix(currentURI.Path,"/") ){
				//root based relative urls
				 log.Debug("old relatived url,",currentUrlStr)
				 currentUrlStr:=seedURI.Host+currentUrlStr;
				 log.Debug("new relatived url,",currentUrlStr)
			}else{
				 log.Debug("old relatived url,",currentUrlStr)
				 //page based relative urls
				 urlPath:=getRootUrl(currentURI)
				 currentUrlStr:=urlPath+currentUrlStr
				 log.Debug("new relatived url,",currentUrlStr)
			 }
			}else{
				log.Debug("host:",currentURI.Host," ",currentURI.Host=="")

				//resolve domain specific filter
				if(siteConfig.FollowSameDomain){

					if(siteConfig.FollowSubDomain){

						//TODO handler com.cn and .com,using a TLC-domain list

					}else if(seedURI.Host !=currentURI.Host){
						log.Debug("domain mismatch,",seedURI.Host," vs ",currentURI.Host)
						continue;
					}
				}
			}




			if(len(siteConfig.LinkUrlMustContain)>0){
				if(!stringutil.ContainStr(currentUrlStr,siteConfig.LinkUrlMustContain)){
					log.Debug("link does not hit must-contain,ignore,",currentUrlStr," , ",siteConfig.LinkUrlMustNotContain)
					continue;
				}
			}

			if(len(siteConfig.LinkUrlMustNotContain)>0){
				if(stringutil.ContainStr(currentUrlStr,siteConfig.LinkUrlMustNotContain)){
					log.Debug("link hit must-not-contain,ignore,",currentUrlStr," , ",siteConfig.LinkUrlMustNotContain)
					continue;
				}
			}

			log.Info("enqueue:",string(url))
			curl <- []byte(currentUrlStr)
			f.Add([]byte(filterUrl))
		}else{
			log.Debug("hit bloom filter,ignore,",string(url))
		}


		//TODO 判断url是否已经请求过，并且判断url pattern，如果满足处理条件，则继续进行处理，否则放弃

	}
}
