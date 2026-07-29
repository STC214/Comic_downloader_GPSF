param(
    [ValidateSet('test', 'vet', 'run')]
    [string]$Mode = 'test',
    [switch]$Playwright,
    [ValidateRange(128, 8192)]
    [int]$MemoryLimitMiB = 2048,
    [string[]]$Packages = @('./...'),
    [switch]$SelfTest
)

$ErrorActionPreference = 'Stop'

if (-not ('BoundedTestJob.NativeMethods' -as [type])) {
    Add-Type @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;

namespace BoundedTestJob {
    [StructLayout(LayoutKind.Sequential)]
    internal struct IO_COUNTERS {
        internal ulong ReadOperationCount;
        internal ulong WriteOperationCount;
        internal ulong OtherOperationCount;
        internal ulong ReadTransferCount;
        internal ulong WriteTransferCount;
        internal ulong OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    internal struct JOBOBJECT_BASIC_LIMIT_INFORMATION {
        internal long PerProcessUserTimeLimit;
        internal long PerJobUserTimeLimit;
        internal uint LimitFlags;
        internal UIntPtr MinimumWorkingSetSize;
        internal UIntPtr MaximumWorkingSetSize;
        internal uint ActiveProcessLimit;
        internal UIntPtr Affinity;
        internal uint PriorityClass;
        internal uint SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    internal struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
        internal JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
        internal IO_COUNTERS IoInfo;
        internal UIntPtr ProcessMemoryLimit;
        internal UIntPtr JobMemoryLimit;
        internal UIntPtr PeakProcessMemoryUsed;
        internal UIntPtr PeakJobMemoryUsed;
    }

    public static class NativeMethods {
        private const uint JOB_OBJECT_LIMIT_JOB_MEMORY = 0x00000200;
        private const uint JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000;
        private const int JobObjectExtendedLimitInformation = 9;

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
        public static extern IntPtr CreateJobObject(IntPtr attributes, string name);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool SetInformationJobObject(
            IntPtr job,
            int infoClass,
            ref JOBOBJECT_EXTENDED_LIMIT_INFORMATION info,
            uint length);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool QueryInformationJobObject(
            IntPtr job,
            int infoClass,
            out JOBOBJECT_EXTENDED_LIMIT_INFORMATION info,
            uint length,
            IntPtr returnLength);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);

        [DllImport("kernel32.dll")]
        public static extern bool CloseHandle(IntPtr handle);

        public static void ConfigureMemoryLimit(IntPtr job, ulong bytes) {
            var info = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION();
            info.BasicLimitInformation.LimitFlags =
                JOB_OBJECT_LIMIT_JOB_MEMORY | JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
            info.JobMemoryLimit = new UIntPtr(bytes);
            uint size = (uint)Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION));
            if (!SetInformationJobObject(
                    job, JobObjectExtendedLimitInformation, ref info, size)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
        }

        public static ulong PeakJobMemory(IntPtr job) {
            JOBOBJECT_EXTENDED_LIMIT_INFORMATION info;
            uint size = (uint)Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION));
            if (!QueryInformationJobObject(
                    job, JobObjectExtendedLimitInformation, out info, size, IntPtr.Zero)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            return info.PeakJobMemoryUsed.ToUInt64();
        }
    }
}
'@
}

$limitBytes = [uint64]$MemoryLimitMiB * 1MB
$job = [BoundedTestJob.NativeMethods]::CreateJobObject([IntPtr]::Zero, $null)
if ($job -eq [IntPtr]::Zero) {
    throw 'CreateJobObject failed'
}

try {
    [BoundedTestJob.NativeMethods]::ConfigureMemoryLimit($job, $limitBytes)
    if ($SelfTest) {
        $inner = @'
$ErrorActionPreference = 'Stop'
Start-Sleep -Milliseconds 500
$blocks = [Collections.Generic.List[byte[]]]::new()
for ($i = 0; $i -lt 32; $i++) {
    $blocks.Add([byte[]]::new(16MB))
}
exit 0
'@
    } else {
        $testArgs = @($Mode)
        if ($Mode -eq 'vet') {
            $testArgs += '-unsafeptr=false'
        }
        $testArgs += @('-p', '1')
        if ($Playwright) {
            $testArgs += @('-tags', 'playwright')
        }
        $testArgs += $Packages
        $quotedArgs = $testArgs | ForEach-Object { "'" + $_.Replace("'", "''") + "'" }
        $inner = @"
`$env:GOMEMLIMIT = '1GiB'
`$env:GOMAXPROCS = '4'
Start-Sleep -Milliseconds 500
& go.exe $($quotedArgs -join ' ')
exit `$LASTEXITCODE
"@
    }

    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($inner))
    $process = Start-Process -FilePath 'powershell.exe' `
        -ArgumentList '-NoProfile', '-EncodedCommand', $encoded `
        -NoNewWindow -PassThru
    if (-not [BoundedTestJob.NativeMethods]::AssignProcessToJobObject($job, $process.Handle)) {
        $process.Kill()
        throw "AssignProcessToJobObject failed: $([Runtime.InteropServices.Marshal]::GetLastWin32Error())"
    }
    $process.WaitForExit()
    $peakMiB = [math]::Round([BoundedTestJob.NativeMethods]::PeakJobMemory($job) / 1MB, 1)
    Write-Output "peak test job committed memory: ${peakMiB}MiB (limit ${MemoryLimitMiB}MiB)"
    if ($SelfTest) {
        if ($process.ExitCode -eq 0) {
            throw 'memory-limit self-test unexpectedly completed'
        }
        Write-Output "memory-limit self-test passed at ${MemoryLimitMiB}MiB"
        return
    }
    if ($process.ExitCode -ne 0) {
        throw "bounded go test failed with exit code $($process.ExitCode)"
    }
} finally {
    [void][BoundedTestJob.NativeMethods]::CloseHandle($job)
}
