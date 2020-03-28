package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/cinode/go/blenc"
	"github.com/cinode/go/datastore"
	"github.com/cinode/go/structure"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func compileCmd() *cobra.Command {

	var srcDir, dstDir string

	cmd := &cobra.Command{
		Use:   "compile --source <src_dir> --destination <dst_dir>",
		Short: "Compile datastore from static files",
		Long: `
The compile command can be used to create an encrypted datastore from
a content with static files that can then be used to serve through a
simple http server.

Files stored on disk are encrypted through the blenc layer. However
this tool should not be considered secure since the encryption key
for the root node is stored in plaintext in an 'entrypoint.txt' file.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if srcDir == "" || dstDir == "" {
				cmd.Help()
				return
			}
			compile(srcDir, dstDir)
		},
	}

	cmd.Flags().StringVarP(&srcDir, "source", "s", "", "Source directory with content to compile")
	cmd.Flags().StringVarP(&dstDir, "destination", "d", "", "Destination directory for blobs")

	return cmd
}

func compile(srcDir, dstDir string) {

	be := blenc.FromDatastore(datastore.InFileSystem(dstDir))

	name, key, err := compileOneLevel(srcDir, be)
	if err != nil {
		log.Fatal(err)
	}

	fl, err := os.Create(path.Join(dstDir, "entrypoint.txt"))
	if err != nil {
		log.Fatal(err)
	}
	_, err = fmt.Fprintf(fl, "%s\n%s\n", name, key)
	if err != nil {
		fl.Close()
		log.Fatal(err)
	}

	err = fl.Close()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("DONE")
}

func compileOneLevel(path string, be blenc.BE) (string, string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("Couldn't check path: %w", err)
	}

	if st.IsDir() {
		return compileDir(path, be)
	}

	if st.Mode().IsRegular() {
		return compileFile(path, be)
	}

	return "", "", fmt.Errorf("Neither dir nor a regular file: %v", path)
}

func compileFile(path string, be blenc.BE) (string, string, error) {
	fmt.Println(" *", path)
	fl, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read file %v: %w", path, err)
	}
	return be.Save(fl, blenc.ContentsHashKey())
}

func compileDir(p string, be blenc.BE) (string, string, error) {
	fileList, err := ioutil.ReadDir(p)
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read contents of dir %v: %w", p, err)
	}
	dirStruct := structure.Directory{
		Entries: make(map[string]*structure.Directory_Entry),
	}
	for _, e := range fileList {
		subPath := path.Join(p, e.Name())
		name, key, err := compileOneLevel(subPath, be)
		if err != nil {
			return "", "", err
		}
		contentType := "application/cinode-dir"
		if !e.IsDir() {
			contentType = mime.TypeByExtension(filepath.Ext(e.Name()))
			if contentType == "" {
				file, err := os.Open(subPath)
				if err != nil {
					return "", "", fmt.Errorf("Can not detect content type for %v: %w", subPath, err)
				}
				buffer := make([]byte, 512)
				n, err := io.ReadFull(file, buffer)
				file.Close()
				if err != nil && err != io.ErrUnexpectedEOF {
					return "", "", fmt.Errorf("Can not detect content type for %v: %w", subPath, err)
				}
				contentType = http.DetectContentType(buffer[:n])
			}
		}
		dirStruct.Entries[e.Name()] = &structure.Directory_Entry{
			Bid:      name,
			Key:      key,
			MimeType: contentType,
		}
	}

	data, err := proto.Marshal(&dirStruct)
	if err != nil {
		return "", "", fmt.Errorf("Can not serialize directory %v: %w", p, err)
	}

	return be.Save(ioutil.NopCloser(bytes.NewReader(data)), blenc.ContentsHashKey())
}
