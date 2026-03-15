// usbgadget/gokrazy.go
package usbgadget

// OTGConfigContent retourne le contenu de la config OTG pour le Pi Zero 2W.
// À copier dans PackageConfig ExtraFileContents pour activer le port USB en mode peripheral.
func OTGConfigContent() string {
	return "dtoverlay=dwc2,dr_mode=peripheral\n"
}
