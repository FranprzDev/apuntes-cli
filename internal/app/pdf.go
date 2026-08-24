package app

import (
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// summaryToPDF renders the session summary into a simple PDF next to the
// markdown file, and returns the PDF path.
func summaryToPDF(s *Session, mdPath string) (string, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.MultiCell(0, 10, tr("Resumen de clase: "+s.Topic), "", "L", false)
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, tr(fmt.Sprintf("Inicio: %s   Fin: %s   Preguntas: %d", s.Start, s.End, len(s.Entries))), "", "L", false)
	pdf.Ln(4)

	for _, e := range s.Entries {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.MultiCell(0, 7, tr("• "+e.Question), "", "L", false)
		pdf.SetFont("Helvetica", "", 11)
		answer := e.Answer
		if answer == "" {
			answer = "(sin respuesta registrada)"
		}
		pdf.MultiCell(0, 6, tr(answer), "", "L", false)
		pdf.Ln(2)
	}

	dest := strings.TrimSuffix(mdPath, ".md") + ".pdf"
	if err := pdf.OutputFileAndClose(dest); err != nil {
		return "", err
	}
	return dest, nil
}
