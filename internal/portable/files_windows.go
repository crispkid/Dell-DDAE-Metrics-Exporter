package portable

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
)

func platformReady() error {
	v := windows.RtlGetVersion()
	if v.MajorVersion != 10 || v.BuildNumber < 22000 || v.ProductType != 1 {
		return ErrInput
	}
	return nil
}
func platformPath(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrInput
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return ErrInput
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return ErrInput
	}
	return nil
}
func protect(path string, dir bool) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ErrStorage
	}
	path = absolute
	volume := filepath.VolumeName(path)
	if volume == "" || strings.HasPrefix(volume, `\\`) {
		return ErrStorage
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return ErrStorage
	}
	var fs [64]uint16
	if windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))) != nil || windows.UTF16ToString(fs[:]) != "NTFS" {
		return ErrStorage
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ErrStorage
	}
	inherit := ""
	if dir {
		inherit = "OICI"
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;" + inherit + ";FA;;;SY)(A;" + inherit + ";FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return ErrStorage
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return ErrStorage
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
