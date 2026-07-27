package cmd

import (
	"fmt"
	"os"

	ocidelta "github.com/containers/oci-delta/pkg/oci-delta"
	tardiff "github.com/containers/tar-diff/pkg/tar-diff"
	"github.com/spf13/cobra"
)

var (
	createVerbose           bool
	createDebug             bool
	createParallelism       int
	createSignatures        []string
	createBinaryDiff        string
	createCompressionLevel  int
	createZstdDiffLevel     int
	createZstdDiffWindowMiB int
	createZstdDiffFallback  bool
)

var createCmd = &cobra.Command{
	Use:   "create [OPTIONS] <old-image> <new-image> <output>",
	Short: "Create a delta between two OCI images",
	Long: `Create a delta between two OCI images.

Arguments:
  <old-image>   Old image (oci-archive:path, oci:path, or containers-storage:ref)
  <new-image>   New image (oci-archive:path, oci:path, or containers-storage:ref)
  <output>      Output delta (oci-archive:path or oci:path)

If no type prefix is given, oci-archive: is assumed.`,
	Args: cobra.ExactArgs(3),
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().BoolVarP(&createVerbose, "verbose", "v", false, "show statistics after creation")
	createCmd.Flags().BoolVar(&createDebug, "debug", false, "show detailed progress information")
	createCmd.Flags().IntVarP(&createParallelism, "jobs", "j", 0, "max parallel tar-diff workers (default: number of CPUs)")
	createCmd.Flags().StringVar(&createBinaryDiff, "binary-diff", "bsdiff", "per-file binary diff method: bsdiff or zstd")
	createCmd.Flags().IntVar(&createCompressionLevel, "compression-level", 0, "outer tar-diff zstd level (0 = tar-diff default)")
	createCmd.Flags().IntVar(&createZstdDiffLevel, "zstd-diff-level", -1, "zstd level for dictionary patches (-1 = use compression-level/default)")
	createCmd.Flags().IntVar(&createZstdDiffWindowMiB, "zstd-diff-window", 0, "zstd window size in MiB for dictionary patches (0 = auto)")
	createCmd.Flags().BoolVar(&createZstdDiffFallback, "zstd-diff-fallback-raw", true, "emit raw data when a zstd dictionary patch is not smaller")
	createCmd.Flags().StringArrayVar(&createSignatures, "signature", nil, "signature OCI artifact to embed (can be specified multiple times)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	tmpDir, err := os.MkdirTemp("/var/tmp", "oci-delta-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	log := &cmdLogger{debug: createDebug}

	log.Debug("Opening old image: %s", args[0])

	oldReader, err := ocidelta.OpenOCIReader(args[0], tmpDir, log)
	if err != nil {
		return fmt.Errorf("failed to open old image: %w", err)
	}
	defer oldReader.Close()

	log.Debug("Opening new image: %s", args[1])

	newReader, err := ocidelta.OpenOCIReader(args[1], tmpDir, log)
	if err != nil {
		return fmt.Errorf("failed to open new image: %w", err)
	}
	defer newReader.Close()

	sigReaders := ocidelta.ExtractedSignatures(newReader)

	for _, sigPath := range createSignatures {
		log.Debug("Opening signature: %s", sigPath)

		sigReader, err := ocidelta.OpenOCIReader(sigPath, tmpDir, log)
		if err != nil {
			return fmt.Errorf("failed to open signature %s: %w", sigPath, err)
		}
		defer sigReader.Close()
		sigReaders = append(sigReaders, sigReader)
	}

	writer, err := ocidelta.OpenOCIWriter(args[2])
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}

	binaryDiffMethod, err := parseBinaryDiffMethod(createBinaryDiff)
	if err != nil {
		return err
	}
	if createZstdDiffWindowMiB < 0 {
		return fmt.Errorf("invalid --zstd-diff-window %d", createZstdDiffWindowMiB)
	}

	zstdDiffLevel := createZstdDiffLevel
	zstdDiffWindow := createZstdDiffWindowMiB
	fallbackRaw := createZstdDiffFallback
	stats, err := ocidelta.CreateDelta(oldReader, newReader, writer, ocidelta.CreateOptions{
		TmpDir:              tmpDir,
		Parallelism:         createParallelism,
		Signatures:          sigReaders,
		BinaryDiffMethod:    binaryDiffMethod,
		CompressionLevel:    createCompressionLevel,
		ZstdDiffLevel:       &zstdDiffLevel,
		ZstdDiffWindowMiB:   &zstdDiffWindow,
		ZstdDiffFallbackRaw: &fallbackRaw,
	}, log)
	if err != nil {
		writer.Close()
		return err
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if createVerbose && stats != nil {
		fmt.Printf("\nDelta creation statistics:\n")
		fmt.Printf("  Old image layers: %d\n", stats.OldLayers)
		fmt.Printf("  New image layers: %d\n", stats.NewLayers)
		fmt.Printf("  Processed layers: %d\n", stats.ProcessedLayers)
		fmt.Printf("  Skipped layers:   %d\n", stats.SkippedLayers)
		fmt.Printf("  Processed layer bytes:  %d\n", stats.ProcessedLayerBytes)
		fmt.Printf("  Tar-diff layer bytes:   %d\n", stats.TarDiffLayerBytes)
		fmt.Printf("  Original layer bytes:   %d\n", stats.OriginalLayerBytes)

		if stats.ProcessedLayerBytes > 0 {
			saved := stats.ProcessedLayerBytes - stats.TarDiffLayerBytes - stats.OriginalLayerBytes
			pct := float64(saved) / float64(stats.ProcessedLayerBytes) * 100
			fmt.Printf("  Bytes saved:            %d (%.1f%%)\n", saved, pct)
		}
	}

	return nil
}

func parseBinaryDiffMethod(value string) (tardiff.BinaryDiffMethod, error) {
	switch value {
	case "bsdiff":
		return tardiff.BinaryDiffBsdiff, nil
	case "zstd":
		return tardiff.BinaryDiffZstd, nil
	default:
		return 0, fmt.Errorf("invalid --binary-diff %q (want bsdiff or zstd)", value)
	}
}
