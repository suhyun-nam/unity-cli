using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using UnityEditor;
using UnityEditor.TestTools.TestRunner.Api;
using UnityEngine;
using Object = UnityEngine.Object;

namespace UnityCliConnector.TestRunner
{
    /// <summary>
    /// Survives domain reloads via [InitializeOnLoad].
    /// Re-registers TestRunnerApi callbacks after PlayMode domain reload
    /// so RunFinished still fires and results are written to file.
    /// 대기·결과 파일은 이 프로젝트 폴더(RunTests.ProjectStatusDir)에만 둔다.
    /// </summary>
    [InitializeOnLoad]
    public static class TestRunnerState
    {
        static TestRunnerState()
        {
            // Asset Import Worker는 테스트를 돌리지 않는다. 같은 프로젝트 폴더의 대기 파일에
            // 콜백을 걸 이유가 없다.
            if (AssetDatabase.IsAssetImportWorkerProcess()) return;

            AssemblyReloadEvents.afterAssemblyReload += OnAfterAssemblyReload;
        }

        public static void MarkPending(string runId, string filter)
        {
            var pending = new { runId, filter = filter ?? "" };
            try
            {
                Directory.CreateDirectory(RunTests.ProjectStatusDir);
                File.WriteAllText(PendingFilePath(runId), JsonConvert.SerializeObject(pending));
            }
            catch { }
        }

        public static void ClearPending(string runId)
        {
            try
            {
                var path = PendingFilePath(runId);
                if (File.Exists(path)) File.Delete(path);
            }
            catch { }
        }

        // 리로드 전에 건 콜백은 사라지므로, 이 프로젝트의 대기 파일마다 다시 건다.
        // 다른 프로젝트 폴더는 보지 않는다. 보면 그 에디터의 실행 ID로 이 에디터의 결과를 쓰게 된다.
        static void OnAfterAssemblyReload()
        {
            try
            {
                var dir = RunTests.ProjectStatusDir;
                Directory.CreateDirectory(dir);
                foreach (var file in Directory.GetFiles(dir, "test-pending-*.json"))
                {
                    var json = File.ReadAllText(file);
                    var pending = JObject.Parse(json);
                    var runId  = pending["runId"]?.Value<string>();
                    var filter = pending["filter"]?.Value<string>();

                    if (string.IsNullOrEmpty(runId)) continue;

                    ReattachCallbacks(runId, filter);
                }
            }
            catch { }
        }

        static void ReattachCallbacks(string runId, string filter)
        {
            var passed  = new List<string>();
            var failed  = new List<string>();
            var skipped = new List<string>();

            var api = ScriptableObject.CreateInstance<TestRunnerApi>();
            var callbacks = new RunTests.TestCallbacks(
                onResult: r => RunTests.CollectResult(r, passed, failed, skipped),
                onFinished: _ =>
                {
                    Object.DestroyImmediate(api);
                    ClearPending(runId);
                    RunTests.WriteResultsFile(runId, passed, failed, skipped);
                }
            );

            api.RegisterCallbacks(callbacks);
        }

        static string PendingFilePath(string runId) =>
            Path.Combine(RunTests.ProjectStatusDir, $"test-pending-{runId}.json");
    }
}
